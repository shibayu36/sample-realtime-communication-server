package game

import (
	"github.com/shibayu36/sample-realtime-communication-server/shared"
)

type ItemType string

const (
	ItemTypeBullet ItemType = "bullet"
)

// ToSharedItemType ItemTypeをshared.ItemTypeに変換する
func (t ItemType) ToSharedItemType() shared.ItemType {
	switch t {
	case ItemTypeBullet:
		return shared.ItemType_BULLET
	default:
		panic("invalid item type: " + string(t))
	}
}

type Item interface {
	collidable
	ID() ItemID
	Type() ItemType
	Position() Position
	Update(provider gameOperationProvider) bool
}
