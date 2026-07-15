// Command rhythmchart converts a Clone Hero .chart file into the static chart
// table consumed by the rhythm demo.
package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	sectionSong         = "Song"
	sectionSyncTrack    = "SyncTrack"
	sectionExpertSingle = "ExpertSingle"
	sectionMediumSingle = "MediumSingle"

	sourceLaneCount = 5
	chartOpenFret   = 7

	// Frets 5 and 6 in a .chart note line are the forced-HOPO and tap
	// modifiers, not playable notes. Hand-authored charts use them heavily.
	chartForcedFlag = 5
	chartTapFlag    = 6
)

var (
	sectionPattern    = regexp.MustCompile(`^\s*\[([^]]+)\]\s*$`)
	resolutionPattern = regexp.MustCompile(`^\s*Resolution\s*=\s*(\d+)\s*$`)
	offsetPattern     = regexp.MustCompile(`^\s*Offset\s*=\s*(-?[0-9]+(?:\.[0-9]+)?)\s*$`)
	tempoPattern      = regexp.MustCompile(`^\s*(\d+)\s*=\s*B\s+(\d+)\s*$`)
	notePattern       = regexp.MustCompile(`^\s*(\d+)\s*=\s*N\s+(\d+)\s+(\d+)\s*$`)
	specialPattern    = regexp.MustCompile(`^\s*(\d+)\s*=\s*S\s+\d+\s+(\d+)\s*$`)
)

var errNoDifficultyNotes = errors.New("no notes in selected difficulty")

type sourceChart struct {
	difficulty string
	resolution int64
	offsetMS   int64 // [Song] Offset in ms: shifts every note relative to the audio
	tempos     []tempoEvent
	notes      []sourceNote
	specials   []sourceSpecial
}

type tempoEvent struct {
	tick     int64
	bpmMilli int64
}

type sourceNote struct {
	tick   int64
	fret   int64
	length int64
}

// sourceSpecial is a Star Power phrase. It is an interval used to mark each
// playable gem it covers as an objectSpecial in the generated chart.
type sourceSpecial struct {
	tick   int64
	length int64
}

type gameNote struct {
	startMS int64
	endMS   int64
	lane    int64
	hold    bool
	special bool
}

type gameSpecial struct {
	startMS int64
	endMS   int64
}

func main() {
	chartPath := flag.String("chart", "", "input .chart file")
	outPath := flag.String("out", "", "output .go file")
	packageName := flag.String("package", "rhythm", "package name for generated Go")
	maxMS := flag.Int64("max-ms", 0, "drop notes after this audio duration in milliseconds (0 keeps all notes)")
	minGapMS := flag.Int64("min-gap-ms", 0, "discard notes within this many milliseconds of the last retained note (0 preserves chart density)")
	synthHoldMS := flag.Int64("synth-hold-min-gap-ms", 0, "turn a tap followed by at least this much silence into a hold (0 disables; onset-only charters never emit sustains)")
	difficulty := flag.String("difficulty", "medium", "chart difficulty: medium (the demo currently supports Medium only)")
	flag.Parse()

	if *chartPath == "" || *outPath == "" {
		fmt.Fprintln(os.Stderr, "rhythmchart: -chart and -out are required")
		os.Exit(2)
	}
	if *maxMS < 0 {
		fmt.Fprintln(os.Stderr, "rhythmchart: -max-ms must not be negative")
		os.Exit(2)
	}
	if *minGapMS < 0 {
		fmt.Fprintln(os.Stderr, "rhythmchart: -min-gap-ms must not be negative")
		os.Exit(2)
	}
	if *synthHoldMS < 0 {
		fmt.Fprintln(os.Stderr, "rhythmchart: -synth-hold-min-gap-ms must not be negative")
		os.Exit(2)
	}
	if *difficulty != "medium" {
		fmt.Fprintln(os.Stderr, "rhythmchart: only -difficulty medium is supported; Hard and Expert require the future five-lane demo")
		os.Exit(2)
	}
	if err := run(*chartPath, *outPath, *packageName, *difficulty, *maxMS, *minGapMS, *synthHoldMS); err != nil {
		fmt.Fprintln(os.Stderr, "rhythmchart:", err)
		os.Exit(1)
	}
}

func run(chartPath, outPath, packageName, difficulty string, maxMS, minGapMS, synthHoldMS int64) error {
	in, err := os.Open(chartPath)
	if err != nil {
		return err
	}
	defer func(in *os.File) {
		err := in.Close()
		if err != nil {
			fmt.Printf("error %s", err)
		}
	}(in)

	chart, err := parseDifficulty(in, difficulty)
	if err != nil {
		return fmt.Errorf("parse %s: %w", chartPath, err)
	}
	notes, err := convert(chart, maxMS, minGapMS)
	if err != nil {
		return err
	}
	notes = synthesizeHolds(notes, synthHoldMS)
	notes = removeNotesDuringHolds(notes)
	specials, err := convertSpecials(chart, maxMS)
	if err != nil {
		return err
	}
	notes = markSpecialNotes(notes, specials)
	data, err := render(packageName, filepath.Base(chartPath), chart.difficulty, notes, minGapMS, synthHoldMS)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outPath, data, 0o644)
}

func parseDifficulty(r io.Reader, difficulty string) (sourceChart, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return sourceChart{}, err
	}
	chart, err := parseDifficultyOnce(bytes.NewReader(data), difficulty)
	if err == nil || difficulty != "medium" || !errors.Is(err, errNoDifficultyNotes) {
		return chart, err
	}

	fallback, fallbackErr := parseDifficultyOnce(bytes.NewReader(data), "expert")
	if fallbackErr != nil {
		return sourceChart{}, fmt.Errorf("medium missing and ExpertSingle fallback failed: %w", fallbackErr)
	}
	fallback.difficulty = "expert-adapted"
	return fallback, nil
}

func parseDifficultyOnce(r io.Reader, difficulty string) (sourceChart, error) {
	noteSection, err := difficultySection(difficulty)
	if err != nil {
		return sourceChart{}, err
	}
	chart := sourceChart{difficulty: difficulty}
	section := ""
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if lineNo == 1 {
			line = strings.TrimPrefix(line, "\ufeff")
		}
		if match := sectionPattern.FindStringSubmatch(line); match != nil {
			section = match[1]
			continue
		}
		switch section {
		case sectionSong:
			if match := resolutionPattern.FindStringSubmatch(line); match != nil {
				value, err := parseInt(match[1])
				if err != nil || value <= 0 {
					return sourceChart{}, fmt.Errorf("line %d: invalid Resolution", lineNo)
				}
				chart.resolution = value
			}
			if match := offsetPattern.FindStringSubmatch(line); match != nil {
				seconds, err := strconv.ParseFloat(match[1], 64)
				if err != nil {
					return sourceChart{}, fmt.Errorf("line %d: invalid Offset", lineNo)
				}
				chart.offsetMS = int64(math.Round(seconds * 1000))
			}
		case sectionSyncTrack:
			if match := tempoPattern.FindStringSubmatch(line); match != nil {
				tick, err := parseInt(match[1])
				if err != nil {
					return sourceChart{}, fmt.Errorf("line %d: invalid tempo tick", lineNo)
				}
				bpmMilli, err := parseInt(match[2])
				if err != nil || bpmMilli <= 0 {
					return sourceChart{}, fmt.Errorf("line %d: invalid BPM", lineNo)
				}
				chart.tempos = append(chart.tempos, tempoEvent{tick: tick, bpmMilli: bpmMilli})
			}
		case noteSection:
			if match := notePattern.FindStringSubmatch(line); match != nil {
				tick, err := parseInt(match[1])
				if err != nil {
					return sourceChart{}, fmt.Errorf("line %d: invalid note tick", lineNo)
				}
				fret, err := parseInt(match[2])
				if err != nil {
					return sourceChart{}, fmt.Errorf("line %d: invalid fret %q", lineNo, match[2])
				}
				if fret == chartForcedFlag || fret == chartTapFlag {
					continue
				}
				if (fret < 0 || fret >= sourceLaneCount) && fret != chartOpenFret {
					return sourceChart{}, fmt.Errorf("line %d: unsupported fret %q", lineNo, match[2])
				}
				length, err := parseInt(match[3])
				if err != nil {
					return sourceChart{}, fmt.Errorf("line %d: invalid note length", lineNo)
				}
				chart.notes = append(chart.notes, sourceNote{tick: tick, fret: fret, length: length})
			}
			if match := specialPattern.FindStringSubmatch(line); match != nil {
				tick, err := parseInt(match[1])
				if err != nil {
					return sourceChart{}, fmt.Errorf("line %d: invalid special tick", lineNo)
				}
				length, err := parseInt(match[2])
				if err != nil || length <= 0 {
					return sourceChart{}, fmt.Errorf("line %d: invalid special length", lineNo)
				}
				chart.specials = append(chart.specials, sourceSpecial{tick: tick, length: length})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return sourceChart{}, err
	}
	if chart.resolution == 0 {
		return sourceChart{}, errors.New("missing [Song] Resolution")
	}
	if len(chart.notes) == 0 {
		return sourceChart{}, fmt.Errorf("%w: [%s]", errNoDifficultyNotes, noteSection)
	}
	sort.Slice(chart.tempos, func(i, j int) bool { return chart.tempos[i].tick < chart.tempos[j].tick })
	if len(chart.tempos) == 0 || chart.tempos[0].tick != 0 {
		return sourceChart{}, errors.New("[SyncTrack] needs a BPM event at tick 0")
	}
	for i := 1; i < len(chart.tempos); i++ {
		if chart.tempos[i-1].tick == chart.tempos[i].tick {
			return sourceChart{}, fmt.Errorf("duplicate BPM event at tick %d", chart.tempos[i].tick)
		}
	}
	sort.SliceStable(chart.notes, func(i, j int) bool {
		if chart.notes[i].tick != chart.notes[j].tick {
			return chart.notes[i].tick < chart.notes[j].tick
		}
		return chart.notes[i].fret < chart.notes[j].fret
	})
	return chart, nil
}

func difficultySection(difficulty string) (string, error) {
	switch difficulty {
	case "medium":
		return sectionMediumSingle, nil
	case "expert":
		return sectionExpertSingle, nil
	default:
		return "", fmt.Errorf("unsupported difficulty %q", difficulty)
	}
}

func convert(chart sourceChart, maxMS, minGapMS int64) ([]gameNote, error) {
	notes := make([]gameNote, 0, len(chart.notes))
	for _, note := range chart.notes {
		startMS, err := chart.msAt(note.tick)
		if err != nil {
			return nil, err
		}
		startMS += chart.offsetMS
		if startMS < 0 {
			continue
		}
		if maxMS > 0 && startMS > maxMS {
			continue
		}
		endMS := int64(0)
		hold := note.length > 0
		if hold {
			endMS, err = chart.msAt(note.tick + note.length)
			if err != nil {
				return nil, err
			}
			endMS += chart.offsetMS
			if maxMS > 0 && endMS > maxMS {
				endMS = maxMS
			}
			if endMS <= startMS {
				hold = false
				endMS = 0
			}
		}
		lane := outputLane(note.fret)
		gameNote := gameNote{startMS: startMS, endMS: endMS, lane: lane, hold: hold}
		if len(notes) > 0 && notes[len(notes)-1].startMS == gameNote.startMS && notes[len(notes)-1].lane == gameNote.lane {
			if gameNote.endMS > notes[len(notes)-1].endMS {
				notes[len(notes)-1] = gameNote
			}
			continue
		}
		notes = append(notes, gameNote)
	}
	notes = simplify(notes, minGapMS)
	if len(notes) == 0 {
		return nil, errors.New("no notes remain after applying -max-ms")
	}
	return notes, nil
}

func convertSpecials(chart sourceChart, maxMS int64) ([]gameSpecial, error) {
	specials := make([]gameSpecial, 0, len(chart.specials))
	for _, phrase := range chart.specials {
		startMS, err := chart.msAt(phrase.tick)
		if err != nil {
			return nil, err
		}
		startMS += chart.offsetMS
		endMS, err := chart.msAt(phrase.tick + phrase.length)
		if err != nil {
			return nil, err
		}
		endMS += chart.offsetMS
		if startMS < 0 || (maxMS > 0 && startMS > maxMS) {
			continue
		}
		if maxMS > 0 && endMS > maxMS {
			endMS = maxMS
		}
		if endMS > startMS {
			specials = append(specials, gameSpecial{startMS: startMS, endMS: endMS})
		}
	}
	return specials, nil
}

// markSpecialNotes transfers Clone Hero's phrase metadata onto the gems that
// are actually played. This keeps a special note in its lane and lets the demo
// render, judge, and score it like every other chart object.
func markSpecialNotes(notes []gameNote, specials []gameSpecial) []gameNote {
	for i := range notes {
		for _, phrase := range specials {
			if notes[i].startMS >= phrase.startMS && notes[i].startMS <= phrase.endMS {
				notes[i].special = true
				break
			}
		}
	}
	return notes
}

const (
	// synthHoldReleaseMS is the gap left between a synthesized hold's end and
	// the next note, so the player has time to release and re-press.
	synthHoldReleaseMS = 200
	// synthHoldMaxMS caps synthesized holds so section breaks do not produce
	// absurdly long sustains.
	synthHoldMaxMS = 2000
	// synthHoldMinMS is the shortest hold worth synthesizing; below it the
	// note stays a tap.
	synthHoldMinMS = 350
)

// synthesizeHolds turns a tap followed by at least minGapMS of silence into
// a hold ending shortly before the next note. Notes sharing a start time
// extend together, and real sustains from the source chart are preserved.
// Onset-only charter models never emit sustain lengths, so without this the
// output would contain taps only.
func synthesizeHolds(notes []gameNote, minGapMS int64) []gameNote {
	if minGapMS <= 0 {
		return notes
	}
	for i := 0; i < len(notes); {
		next := i
		for next < len(notes) && notes[next].startMS == notes[i].startMS {
			next++
		}
		if next == len(notes) {
			break
		}
		if notes[next].startMS-notes[i].startMS < minGapMS {
			i = next
			continue
		}
		end := notes[next].startMS - synthHoldReleaseMS
		if end > notes[i].startMS+synthHoldMaxMS {
			end = notes[i].startMS + synthHoldMaxMS
		}
		if end-notes[i].startMS >= synthHoldMinMS {
			for k := i; k < next; k++ {
				if !notes[k].hold {
					notes[k].hold = true
					notes[k].endMS = end
				}
			}
		}
		i = next
	}
	return notes
}

// removeNotesDuringHolds keeps each lane playable with the four-key control
// scheme: once a hold begins, no later note on that lane may start before its
// release. Notes on the other lanes are retained, so they can be played while
// the hold key remains down.
func removeNotesDuringHolds(notes []gameNote) []gameNote {
	filtered := make([]gameNote, 0, len(notes))
	heldUntil := make(map[int64]int64)
	for _, note := range notes {
		if note.startMS < heldUntil[note.lane] {
			continue
		}
		filtered = append(filtered, note)
		if note.hold && note.endMS > heldUntil[note.lane] {
			heldUntil[note.lane] = note.endMS
		}
	}
	return filtered
}

func simplify(notes []gameNote, minGapMS int64) []gameNote {
	if minGapMS == 0 {
		return notes
	}

	simplified := make([]gameNote, 0, len(notes))
	for _, note := range notes {
		if len(simplified) > 0 && note.startMS-simplified[len(simplified)-1].startMS < minGapMS {
			continue
		}
		simplified = append(simplified, note)
	}
	return simplified
}

func outputLane(fret int64) int64 {
	if fret == chartOpenFret {
		return 2
	}
	if fret == 4 {
		return 3
	}
	return fret
}

func (chart sourceChart) msAt(tick int64) (int64, error) {
	if tick < 0 {
		return 0, errors.New("negative tick")
	}
	var total big.Rat
	lastTick := int64(0)
	lastTempo := chart.tempos[0].bpmMilli
	for _, tempo := range chart.tempos[1:] {
		if tick <= tempo.tick {
			break
		}
		total.Add(&total, milliseconds(tempo.tick-lastTick, chart.resolution, lastTempo))
		lastTick = tempo.tick
		lastTempo = tempo.bpmMilli
	}
	total.Add(&total, milliseconds(tick-lastTick, chart.resolution, lastTempo))

	numerator := new(big.Int).Set(total.Num())
	denominator := total.Denom()
	numerator.Add(numerator, new(big.Int).Quo(denominator, big.NewInt(2)))
	numerator.Quo(numerator, denominator)
	if !numerator.IsInt64() {
		return 0, fmt.Errorf("tick %d is outside the supported time range", tick)
	}
	return numerator.Int64(), nil
}

func milliseconds(ticks, resolution, bpmMilli int64) *big.Rat {
	// A .chart BPM value is BPM * 1000. The duration of ticks is therefore
	// ticks * 60,000,000 / (resolution * bpmMilli) milliseconds.
	numerator := new(big.Int).Mul(big.NewInt(ticks), big.NewInt(60_000_000))
	denominator := new(big.Int).Mul(big.NewInt(resolution), big.NewInt(bpmMilli))
	return new(big.Rat).SetFrac(numerator, denominator)
}

func render(packageName, sourceName, difficulty string, notes []gameNote, minGapMS, synthHoldMS int64) ([]byte, error) {
	if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(packageName) {
		return nil, fmt.Errorf("invalid package name %q", packageName)
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "// Code generated by rhythmchart from %s; DO NOT EDIT.\n", sourceName)
	fmt.Fprintf(&out, "// Difficulty: %s.\n", difficulty)
	fmt.Fprintln(&out, "// Four-lane output merges source frets 3 and 4 into lane 3, and maps open fret 7 to lane 2.")
	if minGapMS > 0 {
		fmt.Fprintf(&out, "// Simplified with a minimum %dms gap between notes.\n", minGapMS)
	}
	if synthHoldMS > 0 {
		fmt.Fprintf(&out, "// Taps followed by at least %dms of silence were synthesized into holds.\n", synthHoldMS)
	}
	fmt.Fprintf(&out, "package %s\n\n", packageName)
	fmt.Fprintln(&out, "const chartLaneCount = 4")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "var chart = [...]chartObject{")
	objects := make([]gameObject, 0, len(notes))
	for _, note := range notes {
		kind := "objectTap"
		if note.special {
			kind = "objectSpecial"
		} else if note.hold {
			kind = "objectHold"
		}
		objects = append(objects, gameObject{kind: kind, startMS: note.startMS, endMS: note.endMS, lane: note.lane})
	}
	sort.SliceStable(objects, func(i, j int) bool {
		if objects[i].startMS != objects[j].startMS {
			return objects[i].startMS < objects[j].startMS
		}
		return objects[i].kind < objects[j].kind
	})
	for id, object := range objects {
		fmt.Fprintf(&out, "\t{ID: %d, Kind: %s, TimeMS: %d, EndMS: %d, Lane: %d},\n", id, object.kind, object.startMS, object.endMS, object.lane)
	}
	fmt.Fprintln(&out, "}")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "const maxObjects = len(chart)")
	return format.Source(out.Bytes())
}

type gameObject struct {
	kind    string
	startMS int64
	endMS   int64
	lane    int64
}

func parseInt(text string) (int64, error) {
	return strconv.ParseInt(text, 10, 64)
}
