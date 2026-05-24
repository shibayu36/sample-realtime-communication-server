package game

import (
	"math/rand"
	"sync"

	"realtime-communication-server/shared"
)

// Game はサーバー側で保持するゲーム空間の状態を管理する。
// プレイヤーの状態管理を担う。
type Game struct {
	Width  int
	Height int

	Players map[PlayerID]*Player

	mu sync.RWMutex
}

func NewGame(width, height int) *Game {
	return &Game{
		Width:   width,
		Height:  height,
		Players: make(map[PlayerID]*Player),
	}
}

// AddPlayer は新規プレイヤーをランダムな初期位置で追加し、追加されたプレイヤーを返す。
func (g *Game) AddPlayer(playerID PlayerID) *Player {
	g.mu.Lock()
	defer g.mu.Unlock()
	player := &Player{
		PlayerID:  playerID,
		position:  Position{X: rand.Intn(g.Width), Y: rand.Intn(g.Height)},
		direction: DirectionUp,
	}
	g.Players[playerID] = player
	return player
}

func (g *Game) RemovePlayer(playerID PlayerID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.Players, playerID)
}

func (g *Game) MovePlayer(playerID PlayerID, position Position, direction Direction) *Player {
	g.mu.Lock()
	defer g.mu.Unlock()

	player, ok := g.Players[playerID]
	if !ok {
		return nil
	}
	player.Move(position, direction)
	return player
}

func (g *Game) GetPlayers() map[PlayerID]*Player {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return shared.CopyMap(g.Players)
}
