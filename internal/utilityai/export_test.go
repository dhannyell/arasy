package utilityai

// EvaluateOne exposes the reference semantics to external tests.
func (p *Profile) EvaluateOne(inputs []float64, prev OptionID, scores []float64) OptionID {
	return p.evaluateOne(inputs, prev, scores)
}
