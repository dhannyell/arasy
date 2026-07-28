package ease

// Curve maps linear progress to eased progress.
//
// Implementations must return exact values at progress 0 and 1. Curves may
// overshoot outside that interval between the endpoints.
type Curve interface {
	Sample(progress float64) float64
}
