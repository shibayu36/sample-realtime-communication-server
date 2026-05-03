package game

import "sync"

type PlayerStatus string

const (
	PlayerStatusAlive PlayerStatus = "alive"
	PlayerStatusDead  PlayerStatus = "dead"
)

// Player はゲームに参加しているプレイヤーを表す
type Player struct {
	PlayerID PlayerID

	position  Position
	direction Direction
	status    PlayerStatus

	mu sync.RWMutex
}

var _ collidable = (*Player)(nil)

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

func (p *Player) Status() PlayerStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

func (p *Player) Move(position Position, direction Direction) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.status == PlayerStatusDead {
		return
	}
	p.position = position
	p.direction = direction
}

func (p *Player) UpdateStatus(status PlayerStatus) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.status == PlayerStatusDead {
		return
	}
	p.status = status
}

// プレイヤーの前方の座標を取得する
func (p *Player) FowardPosition() Position {
	p.mu.RLock()
	defer p.mu.RUnlock()
	dx, dy := p.direction.ToVector()
	return Position{X: p.position.X + dx, Y: p.position.Y + dy}
}

func (p *Player) OnCollideWith(other collidable, provider gameOperationProvider) bool {
	switch other.(type) {
	case *Bullet:
		// 弾と衝突したらプレイヤーはDEAD
		provider.UpdatePlayerStatus(p.PlayerID, PlayerStatusDead)
		return true
	default:
		return false
	}
}
