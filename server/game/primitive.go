package game

type PlayerID string

// Position はゲーム空間上の座標を表す
type Position struct {
	X int
	Y int
}

type Direction string

const (
	DirectionUp    Direction = "up"
	DirectionDown  Direction = "down"
	DirectionLeft  Direction = "left"
	DirectionRight Direction = "right"
)
