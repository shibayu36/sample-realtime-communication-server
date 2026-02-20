package game

import "sync"

type Bullet struct {
	id        ItemID
	position  Position
	direction Direction
	// 何tickで動くか
	moveTick int
	// 現在のtick
	tick int

	mu sync.RWMutex
}

var _ Item = (*Bullet)(nil)

func NewBullet(id ItemID, position Position, direction Direction) *Bullet {
	return &Bullet{
		id:        id,
		position:  position,
		direction: direction,
		moveTick:  30, // 60fpsで0.5秒
		tick:      0,
	}
}

func (b *Bullet) ID() ItemID {
	return b.id
}

func (b *Bullet) Type() ItemType {
	return ItemTypeBullet
}

func (b *Bullet) Position() Position {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.position
}

func (b *Bullet) Update(_ gameOperationProvider) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tick++
	if b.tick >= b.moveTick {
		b.tick = 0
		switch b.direction {
		case DirectionUp:
			b.position.Y--
		case DirectionDown:
			b.position.Y++
		case DirectionLeft:
			b.position.X--
		case DirectionRight:
			b.position.X++
		}
		return true
	}
	return false
}

func (b *Bullet) OnCollideWith(other collidable, provider gameOperationProvider) bool {
	switch other.(type) {
	case *Player:
		// プレイヤーと衝突したら自分自身は消滅
		provider.RemoveItem(b.ID())
		return true
	default:
		return false
	}
}
