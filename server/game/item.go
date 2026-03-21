package game

import (
	"github.com/shibayu36/sample-realtime-communication-server/shared"
)

type ItemType string

const (
	ItemTypeBullet ItemType = "bullet"
)

func (t ItemType) ToSharedItemType() shared.ItemType {
	switch t {
	case ItemTypeBullet:
		return shared.ItemType_BULLET
	default:
		panic("invalid item type: " + string(t))
	}
}

// Item はゲームループで管理されるゲームオブジェクトを表す
type Item interface {
	collidable
	ID() ItemID
	Type() ItemType
	Position() Position
	// Update はゲームループの1tickごとに呼ばれる。状態が変更された場合はtrueを返す。
	Update(provider gameOperationProvider) bool
}
