package uiManager

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"

	"github.com/dhannyell/frameforge/render"
)

func GenerateLeague(seed uint64) ([]Club, []Fixture) {
	rng := rand.New(rand.NewPCG(seed, 0))
	clubs := make([]Club, len(clubNames))
	for i := range clubs {
		clubs[i] = Club{
			Name:     clubNames[i],
			Short:    clubShorts[i],
			Stadium:  clubStadiums[i],
			Accent:   clubColors[i],
			Strength: 0.5 + rng.Float64()*0.4,
			Squad:    generateSquad(rng),
		}
	}
	return clubs, roundRobin(len(clubs))
}

func SimulateRound(s *ManagerState) {
	if s.Round > len(s.Clubs)*2-2 {
		return
	}
	for _, fx := range fixturesOfRound(s.Fixtures, s.Round) {
		rng := rand.New(rand.NewPCG(s.Seed, uint64(fx.Round)<<18|uint64(fx.Home)<<10|uint64(fx.Away)))
		home := s.Clubs[fx.Home]
		away := s.Clubs[fx.Away]
		homeGoals := poissonSample(rng, 1.25*home.Strength*formFactor(home)+0.35)
		awayGoals := poissonSample(rng, 1.2*away.Strength*formFactor(away))
		s.Results = append(s.Results, Result{Fixture: fx, HomeGoals: homeGoals, AwayGoals: awayGoals})
	}
	s.Round++
}

func Standings(clubs []Club, results []Result) []TableRow {
	rows := make([]TableRow, len(clubs))
	for i := range rows {
		rows[i].Club = i
	}
	for _, result := range results {
		home := &rows[result.Home]
		away := &rows[result.Away]
		home.Played++
		away.Played++
		home.GoalsFor += result.HomeGoals
		home.GoalsAgainst += result.AwayGoals
		away.GoalsFor += result.AwayGoals
		away.GoalsAgainst += result.HomeGoals
		switch {
		case result.HomeGoals > result.AwayGoals:
			home.Wins++
			away.Loss++
			home.Points += 3
		case result.HomeGoals < result.AwayGoals:
			away.Wins++
			home.Loss++
			away.Points += 3
		default:
			home.Draws++
			away.Draws++
			home.Points++
			away.Points++
		}
	}
	slices.SortStableFunc(rows, func(a, b TableRow) int {
		switch {
		case a.Points != b.Points:
			return b.Points - a.Points
		case a.Wins != b.Wins:
			return b.Wins - a.Wins
		case a.GoalDiff() != b.GoalDiff():
			return b.GoalDiff() - a.GoalDiff()
		case a.GoalsFor != b.GoalsFor:
			return b.GoalsFor - a.GoalsFor
		default:
			return a.Club - b.Club
		}
	})
	return rows
}

func NextFixture(s ManagerState, club int) (Fixture, bool) {
	for _, fx := range s.Fixtures {
		if fx.Round < s.Round {
			continue
		}
		if fx.Home == club || fx.Away == club {
			return fx, true
		}
	}
	return Fixture{}, false
}

func LastResult(s ManagerState, club int) (Result, bool) {
	for i := len(s.Results) - 1; i >= 0; i-- {
		result := s.Results[i]
		if result.Home == club || result.Away == club {
			return result, true
		}
	}
	return Result{}, false
}

func Morale(c Club) string {
	if len(c.Squad) == 0 {
		return "REGULAR"
	}
	total := 0.0
	for _, p := range c.Squad {
		total += p.Form
	}
	avg := total / float64(len(c.Squad))
	switch {
	case avg >= 0.78:
		return "EXCELLENT"
	case avg >= 0.62:
		return "GOOD"
	case avg >= 0.45:
		return "REGULAR"
	default:
		return "LOW"
	}
}

func Cash(clubs []Club, results []Result, clubIdx int) string {
	points, losses := 0, 0
	for _, result := range results {
		switch {
		case result.Home == clubIdx && result.HomeGoals > result.AwayGoals:
			points += 3
		case result.Away == clubIdx && result.AwayGoals > result.HomeGoals:
			points += 3
		case result.Home == clubIdx && result.HomeGoals == result.AwayGoals:
			points++
		case result.Away == clubIdx && result.HomeGoals == result.AwayGoals:
			points++
		case result.Home == clubIdx || result.Away == clubIdx:
			losses++
		}
	}
	value := 3.0 + float64(points)*0.15 - float64(losses)*0.08
	if len(clubs) == 0 {
		value = 3.0
	}
	return fmt.Sprintf("$ %.1fM", value)
}

func fixturesOfRound(fixtures []Fixture, round int) []Fixture {
	var out []Fixture
	for _, fx := range fixtures {
		if fx.Round == round {
			out = append(out, fx)
		}
	}
	return out
}

func roundRobin(count int) []Fixture {
	teams := make([]int, count)
	for i := range teams {
		teams[i] = i
	}
	fixtures := make([]Fixture, 0, (count-1)*count)
	for round := 1; round < count; round++ {
		for i := 0; i < count/2; i++ {
			home := teams[i]
			away := teams[count-1-i]
			if round%2 == 0 {
				home, away = away, home
			}
			fixtures = append(fixtures, Fixture{Round: round, Home: home, Away: away})
		}
		teams = append([]int{teams[0], teams[count-1]}, teams[1:count-1]...)
	}
	firstTurn := slices.Clone(fixtures)
	for _, fx := range firstTurn {
		fixtures = append(fixtures, Fixture{Round: fx.Round + count - 1, Home: fx.Away, Away: fx.Home})
	}
	return fixtures
}

func generateSquad(rng *rand.Rand) []Player {
	positions := []Position{GK, GK, CB, CB, CB, LB, RB, DMF, DMF, CMF, CMF, AMF, LMF, RMF, LMF, CF, LWF, RWF, SS}
	squad := make([]Player, len(positions))
	for i, pos := range positions {
		attack := rating(rng, 60, 15)
		defense := rating(rng, 60, 15)
		switch pos {
		case GK, CB, LB, RB:
			defense = clampInt(defense+10, 30, 95)
		case SS, CF, LWF, RWF:
			attack = clampInt(attack+10, 30, 95)
		case DMF, CMF, AMF, LMF, RMF:
			attack = clampInt(attack+4, 30, 95)
			defense = clampInt(defense+4, 30, 95)
		}
		squad[i] = Player{
			Name:    playerNames[rng.IntN(len(playerNames))],
			Pos:     pos,
			Age:     18 + rng.IntN(17),
			Attack:  attack,
			Defense: defense,
			Stamina: rating(rng, 64, 14),
			Form:    0.45 + rng.Float64()*0.45,
		}
	}
	return squad
}

func formFactor(c Club) float64 {
	total := 0.0
	for _, p := range c.Squad {
		total += p.Form
	}
	if len(c.Squad) == 0 {
		return 1
	}
	return 0.75 + total/float64(len(c.Squad))*0.5
}

func poissonSample(rng *rand.Rand, lambda float64) int {
	limit := math.Exp(-lambda)
	k := 0
	p := 1.0
	for p > limit {
		k++
		p *= rng.Float64()
	}
	return k - 1
}

func rating(rng *rand.Rand, mean, spread int) int {
	return clampInt(mean-spread+rng.IntN(spread*2+1), 30, 95)
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

var clubNames = []string{
	"Man Red",
	"London FC",
	"North London",
	"Man Blue",
	"Catalonia",
	"Madrid Rosas",
	"MD White",
	"Tyneisde",
	"Piemonte Calcio",
	"Latium",
	"Merseyside Red",
	"Hampshire Red",
	"East Dorset Chimes",
	"Sevilla Triana",
	"Núñez",
	"Bergamo Calcio",
	"Roma",
	"Merseyside Blue",
	"West Midlands Village",
	"Buenos Aires",
}

var clubShorts = []string{
	"MAR",
	"LON",
	"NOL",
	"MAB",
	"CAT",
	"MDR",
	"MDW",
	"TYN",
	"PCA",
	"LAT",
	"MER",
	"HAM",
	"EAS",
	"SEV",
	"NUN",
	"BER",
	"ROM",
	"MEB",
	"WES",
	"BUE",
}

var clubStadiums = []string{
	"Estadio del Nuevo Triunfo",
	"Alberto J. Armando",
	"Training Ground",
	"National Stadium",
	"Red Cauldron",
	"Magpie Park",
	"Lutecia Park:",
	"Massilia Stadium",
	"Waldstadion",
	"El Libertador",
	"Dromo",
	"Ivy Lane",
	"Crown Lane",
	"Aloha Park",
	"Al Jayeed Stadium",
	"Sanderson Park",
	"Arena D’Oro",
	"Town Park",
	"Stadion Hanguk",
	"Burg Stadio",
}

var clubColors = setClubColors()

func setClubColors() []render.Color {
	colors := make([]render.Color, 0, len(clubNames))
	for i := range clubNames {
		colors = append(colors, brickColor(i))
	}
	return colors
}

var playerNames = []string{
	"Allejo", "Oswaldinato", "Roberto Larcos", "Ravoldi", "Gellazca", "Castolo",
	"Minanda", "Ximelez", "Espimas", "Ivarov", "Gutierrez", "Capitale",
	"Adamelton", "Aksantetro", "Alpanqua", "Jaric", "Iouga", "Ordaz",
}

func brickColor(row int) render.Color {
	const aurea = 0.618033988749895
	s := 0.75
	l := 0.50

	h := math.Mod(float64(row)*aurea, 1.0)

	hue2 := func(p, q, t float64) float64 {
		if t < 0 {
			t += 1
		}
		if t > 1 {
			t -= 1
		}
		if t < 1.0/6.0 {
			return p + (q-p)*6*t
		}
		if t < 1.0/2.0 {
			return q
		}
		if t < 2.0/3.0 {
			return p + (q-p)*(2.0/3.0-t)*6
		}
		return p
	}

	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q

	rFloat := hue2(p, q, h+1.0/3.0)
	gFloat := hue2(p, q, h)
	bFloat := hue2(p, q, h-1.0/3.0)

	return render.Color{R: rFloat, G: gFloat, B: bFloat, A: 1}
}
