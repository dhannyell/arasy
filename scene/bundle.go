package scene

type Bundle struct {
	Transform      *Transform
	CircleSprite   *CircleSprite
	Velocity       *Velocity
	Acceleration   *Acceleration
	Damping        *Damping
	Body           *Body
	CircleCollider *CircleCollider
}
