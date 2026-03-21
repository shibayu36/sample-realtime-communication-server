package game

import (
	"fmt"

	"github.com/shibayu36/sample-realtime-communication-server/shared"
)

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

func (d Direction) ToSharedDirection() shared.Direction {
	switch d {
	case DirectionUp:
		return shared.Direction_UP
	case DirectionDown:
		return shared.Direction_DOWN
	case DirectionLeft:
		return shared.Direction_LEFT
	case DirectionRight:
		return shared.Direction_RIGHT
	default:
		panic(fmt.Sprintf("invalid direction: %s", d))
	}
}

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

func FromSharedDirection(d shared.Direction) (Direction, error) {
	switch d {
	case shared.Direction_UP:
		return DirectionUp, nil
	case shared.Direction_DOWN:
		return DirectionDown, nil
	case shared.Direction_LEFT:
		return DirectionLeft, nil
	case shared.Direction_RIGHT:
		return DirectionRight, nil
	default:
		return "", fmt.Errorf("invalid direction: %d", d)
	}
}
