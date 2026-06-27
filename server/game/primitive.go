package game

type (
	PlayerID string
	ItemID   string
)

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

// 方向を、dxとdyのベクトルに変換する
func (d Direction) ToVector() (int, int) {
	switch d {
	case DirectionUp:
		return 0, -1
	case DirectionDown:
		return 0, 1
	case DirectionLeft:
		return -1, 0
	case DirectionRight:
		return 1, 0
	}
	return 0, 0
}
