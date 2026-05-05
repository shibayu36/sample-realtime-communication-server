package game

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"realtime-communication-server/shared"
)

// Game はサーバー側で保持するゲーム空間の状態を管理する。
// プレイヤー・アイテムの状態管理、ゲームループによる更新、衝突判定を担う。
type Game struct {
	Width  int
	Height int

	Players map[PlayerID]*Player
	Items   map[ItemID]Item

	// 新しく追加されたアイテムを管理する
	// 1tickごとにFlushされる
	AddedItems map[ItemID]Item

	// 削除されたアイテムを管理する
	RemovedItems map[ItemID]Item

	mu sync.RWMutex
}

// gameOperationProvider はアイテム更新や衝突時に必要な操作を提供するインターフェース
type gameOperationProvider interface {
	RemoveItem(id ItemID)
	UpdatePlayerStatus(playerID PlayerID, status PlayerStatus) *Player
}

var _ gameOperationProvider = (*Game)(nil)

func NewGame(width, height int) *Game {
	return &Game{
		Width:        width,
		Height:       height,
		Players:      make(map[PlayerID]*Player),
		Items:        make(map[ItemID]Item),
		AddedItems:   make(map[ItemID]Item),
		RemovedItems: make(map[ItemID]Item),
	}
}

type UpdatedResultType string

const (
	UpdatedResultTypeItemsUpdated   UpdatedResultType = "items_updated"
	UpdatedResultTypePlayersUpdated UpdatedResultType = "players_updated"
)

type UpdatedResult struct {
	Type UpdatedResultType
}

// ゲーム状態を更新するループを開始する
// アイテムが何らか更新されたことを通知するチャネルを返す
func (g *Game) StartUpdateLoop(ctx context.Context) <-chan UpdatedResult {
	updatedCh := make(chan UpdatedResult)

	go func() {
		defer close(updatedCh)

		ticker := time.NewTicker(16700 * time.Microsecond) // 16.7ms ≈ 60fps
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				g.update(updatedCh)
			case <-ctx.Done():
				return
			}
		}
	}()

	return updatedCh
}

// update は1tickごとのゲーム状態更新を行う。
func (g *Game) update(updatedCh chan<- UpdatedResult) {
	items := g.GetItems()

	updatedItems := []Item{}
	updatedPlayers := []*Player{}

	// 各アイテムを1tick進める
	for _, item := range items {
		if item.Update(g) {
			updatedItems = append(updatedItems, item)
		}
	}
	// 盤面外に出たアイテムを削除する
	for _, updatedItem := range updatedItems {
		if !g.isWithinBounds(updatedItem) {
			g.RemoveItem(updatedItem.ID())
		}
	}

	// プレイヤーとアイテムの衝突を検出し、それぞれに衝突時の処理を委譲する
	for _, collision := range g.detectCollisions() {
		if collision.Player.OnCollideWith(collision.Item, g) {
			updatedPlayers = append(updatedPlayers, collision.Player)
		}

		if collision.Item.OnCollideWith(collision.Player, g) {
			updatedItems = append(updatedItems, collision.Item)
		}
	}

	// このtick中に追加されたアイテムを取り込む
	g.mu.Lock()
	for _, item := range g.AddedItems {
		updatedItems = append(updatedItems, item)
	}
	g.AddedItems = make(map[ItemID]Item)
	g.mu.Unlock()

	// 変更があった場合、配信ループに通知する
	if len(updatedItems) > 0 {
		updatedCh <- UpdatedResult{Type: UpdatedResultTypeItemsUpdated}
	}

	if len(updatedPlayers) > 0 {
		updatedCh <- UpdatedResult{Type: UpdatedResultTypePlayersUpdated}
	}
}

// detectCollisions は現在のゲーム状態から衝突しているオブジェクトのペアを検出する
func (g *Game) detectCollisions() []collision {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var collisions []collision

	// アイテムを座標ごとにグルーピング
	itemPosMap := make(map[Position][]Item)
	for _, item := range g.Items {
		itemPosMap[item.Position()] = append(itemPosMap[item.Position()], item)
	}

	// プレイヤーと同じ座標にいるアイテムを衝突として検出
	for _, player := range g.Players {
		for _, item := range itemPosMap[player.Position()] {
			collisions = append(collisions, collision{
				Player: player,
				Item:   item,
			})
		}
	}

	return collisions
}

// アイテムが盤面内にあるかどうかを判定する
func (g *Game) isWithinBounds(item Item) bool {
	pos := item.Position()
	return pos.X >= 0 && pos.X < g.Width && pos.Y >= 0 && pos.Y < g.Height
}

// AddPlayer は新規プレイヤーをランダムな初期位置で追加し、追加されたプレイヤーを返す。
func (g *Game) AddPlayer(playerID PlayerID) *Player {
	g.mu.Lock()
	defer g.mu.Unlock()
	player := &Player{
		PlayerID:  playerID,
		position:  Position{X: rand.Intn(g.Width), Y: rand.Intn(g.Height)},
		direction: DirectionUp,
		status:    PlayerStatusAlive,
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

func (g *Game) UpdatePlayerStatus(playerID PlayerID, status PlayerStatus) *Player {
	g.mu.Lock()
	defer g.mu.Unlock()

	player, ok := g.Players[playerID]
	if !ok {
		return nil
	}
	player.UpdateStatus(status)
	return player
}

func (g *Game) GetPlayers() map[PlayerID]*Player {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return shared.CopyMap(g.Players)
}

func (g *Game) GetItems() map[ItemID]Item {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return shared.CopyMap(g.Items)
}

func (g *Game) GetRemovedItems() map[ItemID]Item {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return shared.CopyMap(g.RemovedItems)
}

func (g *Game) ClearRemovedItem(itemID ItemID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.RemovedItems, itemID)
}

func (g *Game) RemoveItem(itemID ItemID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	item, ok := g.Items[itemID]
	if !ok {
		return
	}
	delete(g.Items, itemID)
	g.RemovedItems[itemID] = item
}

// アイテム追加をLockなしで行う内部メソッド
func (g *Game) addItemWithoutLock(item Item) {
	if g.isWithinBounds(item) {
		g.Items[item.ID()] = item
		g.AddedItems[item.ID()] = item
	}
}

func (g *Game) AddBullet(position Position, direction Direction) ItemID {
	g.mu.Lock()
	defer g.mu.Unlock()
	bullet := NewBullet(ItemID(uuid.New().String()), position, direction)
	g.addItemWithoutLock(bullet)
	return bullet.ID()
}

// ShootBullet はプレイヤーの前方に弾を生成する。弾の移動はゲームループが管理する。
func (g *Game) ShootBullet(playerID PlayerID) ItemID {
	g.mu.Lock()
	defer g.mu.Unlock()

	player, ok := g.Players[playerID]
	if !ok {
		return ItemID("")
	}

	// deadの場合は弾を発射できない
	if player.Status() == PlayerStatusDead {
		return ItemID("")
	}

	// プレイヤーの前方に発射する
	position := player.FowardPosition()
	direction := player.Direction()

	bullet := NewBullet(ItemID(uuid.New().String()), position, direction)
	g.addItemWithoutLock(bullet)

	return bullet.ID()
}
