package game

import (
	"sync"
	"uuid"
)

// 爆風が届く範囲（上下左右それぞれ何マスか）
const fireRange = 3

// Bomb はプレイヤーが置いたボムを表す。サーバーのゲームループで一定時間後に爆発する。
type Bomb struct {
	id       ItemID
	position Position
	// 何tickで爆発するか
	explodeTick int
	// 現在のtick
	tick int

	mu sync.RWMutex
}

var _ Item = (*Bomb)(nil)

func NewBomb(id ItemID, position Position) *Bomb {
	return &Bomb{
		id:          id,
		position:    position,
		explodeTick: 180, // 60fpsで3秒
		tick:        0,
	}
}

func (b *Bomb) ID() ItemID {
	return b.id
}

func (b *Bomb) Type() ItemType {
	return ItemTypeBomb
}

func (b *Bomb) Position() Position {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.position
}

// Update はexplodeTickに達したら爆発し、爆風を配置して自身は消える。
func (b *Bomb) Update(provider gameOperationProvider) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tick++
	if b.tick >= b.explodeTick {
		provider.RemoveItem(b.id)
		for _, pos := range b.firePositions() {
			provider.AddItem(NewBombFire(ItemID(uuid.New().String()), pos))
		}
		return true
	}
	return false
}

// firePositions は自分の位置と上下左右fireRangeマス分の爆風の位置を返す
func (b *Bomb) firePositions() []Position {
	positions := []Position{b.position}

	for _, d := range []Direction{DirectionUp, DirectionDown, DirectionLeft, DirectionRight} {
		dx, dy := d.ToVector()
		for i := 1; i <= fireRange; i++ {
			positions = append(positions, Position{
				X: b.position.X + dx*i,
				Y: b.position.Y + dy*i,
			})
		}
	}

	return positions
}

func (b *Bomb) OnCollideWith(other collidable, provider gameOperationProvider) bool {
	return false
}
