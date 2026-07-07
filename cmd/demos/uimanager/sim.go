package uimanager

import (
	"math"
	"math/rand/v2"
)

// SimulateRound runs all fixtures for the current round and advances Round by 1.
// Deterministic: same seed + same round = same scores, always.
func SimulateRound(s *ManagerState) {
	if s.Round > len(s.Clubs)*2-2 {
		return
	}
	for _, fx := range fixturesOfRound(s.Fixtures, s.Round) {
		rng := rand.New(rand.NewPCG(s.Seed, uint64(fx.Round)<<16|uint64(fx.Home)<<10|uint64(fx.Away)))
		home := s.Clubs[fx.Home]
		away := s.Clubs[fx.Away]
		homeGoals := poissonSample(rng, 1.25*home.Strength*formFactor(home)+0.35)
		awayGoals := poissonSample(rng, 1.2*away.Strength*formFactor(away))
		s.Results = append(s.Results, Result{Fixture: fx, HomeGoals: homeGoals, AwayGoals: awayGoals})
	}
	s.Round++
}

// fixturesOfRound returns all fixtures for the given round number.
func fixturesOfRound(fixtures []Fixture, round int) []Fixture {
	var out []Fixture
	for _, fx := range fixtures {
		if fx.Round == round {
			out = append(out, fx)
		}
	}
	return out
}

// formFactor returns a multiplier (0.75–1.25) based on squad average Form.
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

// poissonSample draws a Poisson-distributed integer via Knuth's method.
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
