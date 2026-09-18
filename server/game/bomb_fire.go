package game

import "sync"

// BombFire はボムの爆風を表す。一定時間が経つと消える。
type BombFire struct {
	id       ItemID
	position Position
	// 何tickで消えるか
	lifeTick int
	// 現在のtick
	tick int

	mu sync.RWMutex
}

var _ Item = (*BombFire)(nil)

func NewBombFire(id ItemID, position Position) *BombFire {
	return &BombFire{
		id:       id,
		position: position,
		lifeTick: 60, // 60fpsで1秒
		tick:     0,
	}
}

func (f *BombFire) ID() ItemID {
	return f.id
}

func (f *BombFire) Type() ItemType {
	return ItemTypeBombFire
}

func (f *BombFire) Position() Position {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.position
}

// Update はlifeTickに達したら消える。
func (f *BombFire) Update(provider gameOperationProvider) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tick++
	if f.tick >= f.lifeTick {
		provider.RemoveItem(f.id)
		return true
	}
	return false
}

func (f *BombFire) OnCollideWith(other collidable, provider gameOperationProvider) bool {
	return false
}
