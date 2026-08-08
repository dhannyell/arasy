// Package utilityai evaluates data-driven utility profiles over batches of
// agents (SoA columns). This is a simulation-path package: results are
// bit-identical across architectures. Transcendentals come from dmath.
package utilityai

// InputID indexes a column in Inputs.
type InputID uint16

// OptionID indexes an option in a Profile.
type OptionID uint16

// NoOption marks an agent with no previous option; it disables the
// commit bonus for that agent.
const NoOption OptionID = 0xFFFF

// Consideration scores one input through one response curve.
type Consideration struct {
	Input InputID
	Curve Curve
}

// Option is one atomic action the profile can choose.
type Option struct {
	Name           string // for trace and overlays; never enters evaluation
	Weight         float64
	Considerations []Consideration
}

// Profile is pure data and serializable. CommitBonus multiplies the score
// of an agent's previous option by (1+CommitBonus) to prevent flip-flopping.
type Profile struct {
	Options     []Option
	CommitBonus float64
}

// evaluateOne scores every option for one agent and returns the best one.
// inputs is indexed by InputID. Ties keep the lowest option index. When
// scores is not nil, it receives one score per option. This is the
// reference semantics; EvaluateBatch must match it agent by agent.
func (p *Profile) evaluateOne(inputs []float64, prev OptionID, scores []float64) OptionID {
	if len(p.Options) == 0 {
		return NoOption
	}
	best := OptionID(0)
	bestScore := -1.0
	for oi := range p.Options {
		s := scoreOption(&p.Options[oi], inputs)
		if OptionID(oi) == prev {
			s *= 1 + p.CommitBonus
		}
		if scores != nil {
			scores[oi] = s
		}
		if s > bestScore {
			bestScore = s
			best = OptionID(oi)
		}
	}
	return best
}

// Inputs are SoA columns: Inputs[input][agent], values already normalized
// to [0,1] by the consumer.
type Inputs [][]float64

// Trace receives the score of every option for every agent. It feeds the
// "why" overlays; the no-trace path stays allocation-free.
type Trace struct {
	Scores [][]float64 // [agent][option]
}

// EvaluateBatch picks the best option for each agent and writes it to
// out (len == number of agents). prev[i] is the agent's current option
// for the commit bonus; a nil prev disables it. The loop is agent-outer,
// so each column is read as a sequential stream and the function needs
// no scratch: with a nil trace it does not allocate.
func (p *Profile) EvaluateBatch(inputs Inputs, prev []OptionID, out []OptionID, trace *Trace) {
	if trace != nil && len(trace.Scores) < len(out) {
		trace.Scores = make([][]float64, len(out))
	}
	for i := range out {
		pr := NoOption
		if prev != nil {
			pr = prev[i]
		}
		var scores []float64
		if trace != nil {
			if len(trace.Scores[i]) < len(p.Options) {
				trace.Scores[i] = make([]float64, len(p.Options))
			}
			scores = trace.Scores[i]
		}
		out[i] = p.evaluateAgent(inputs, i, pr, scores)
	}
}

// evaluateAgent is the column-based twin of evaluateOne. The two are kept
// independent on purpose: TestBatchMatchesOne compares their results.
func (p *Profile) evaluateAgent(inputs Inputs, agent int, prev OptionID, scores []float64) OptionID {
	if len(p.Options) == 0 {
		return NoOption
	}
	best := OptionID(0)
	bestScore := -1.0
	for oi := range p.Options {
		opt := &p.Options[oi]
		s := opt.Weight
		if n := len(opt.Considerations); n > 0 {
			mod := 1.0 - 1.0/float64(n)
			for _, con := range opt.Considerations {
				c := con.Curve.Evaluate(inputs[con.Input][agent])
				makeup := float64((1.0 - c) * mod)
				c += float64(makeup * c)
				s *= c
			}
		}
		if OptionID(oi) == prev {
			s *= 1 + p.CommitBonus
		}
		if scores != nil {
			scores[oi] = s
		}
		if s > bestScore {
			bestScore = s
			best = OptionID(oi)
		}
	}
	return best
}

// scoreOption is Weight times the product of the compensated consideration
// scores. The compensation factor (mod = 1 - 1/n; s += (1-s)*mod*s) keeps
// options with many considerations competitive against options with few.
func scoreOption(opt *Option, inputs []float64) float64 {
	n := len(opt.Considerations)
	score := opt.Weight
	if n == 0 {
		return score
	}
	mod := 1.0 - 1.0/float64(n)
	for _, con := range opt.Considerations {
		s := con.Curve.Evaluate(inputs[con.Input])
		// The float64 casts block FMA fusion in the compensation sum.
		makeup := float64((1.0 - s) * mod)
		s += float64(makeup * s)
		score *= s
	}
	return score
}
