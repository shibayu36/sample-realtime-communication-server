package game

import "sync"

// Player はゲームに参加しているプレイヤーを表す
type Player struct {
	PlayerID PlayerID

	position  Position
	direction Direction

	mu sync.RWMutex
}

func (p *Player) Position() Position {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.position
}

func (p *Player) Direction() Direction {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.direction
}

func (p *Player) Move(position Position, direction Direction) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.position = position
	p.direction = direction
}

// プレイヤーの前方の位置を取得する
func (p *Player) FowardPosition() Position {
	p.mu.RLock()
	defer p.mu.RUnlock()
	dx, dy := p.direction.ToVector()
	return Position{X: p.position.X + dx, Y: p.position.Y + dy}
}
