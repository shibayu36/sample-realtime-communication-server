package main

import (
	"context"
	"fmt"

	"github.com/shibayu36/sample-realtime-communication-server/server/game"
	"github.com/shibayu36/sample-realtime-communication-server/shared"
	"github.com/shibayu36/sample-realtime-communication-server/shared/protocol"
	"google.golang.org/protobuf/proto"
)

// GameService クライアントからのメッセージをゲームの状態に反映し、さらに他のクライアントに状態同期をする役割を持つ
type GameService struct {
	broker *Broker
	game   *game.Game
}

var _ Handler = (*GameService)(nil)

func NewGameService(broker *Broker, game *game.Game) *GameService {
	return &GameService{broker: broker, game: game}
}

// OnConnected はプレイヤーをBrokerとGameに登録し、新規接続プレイヤーへwelcomeを送り、さらに既存のプレイヤーとアイテムの状態を送信する。
// また、新規プレイヤーの存在を他クライアントへ通知する。
func (s *GameService) OnConnected(client *Client) error {
	s.broker.AddClient(client)
	newPlayer := s.game.AddPlayer(game.PlayerID(client.ID()))

	// IDや初期位置などを本人にwelcomeメッセージとして送信する
	newPlayerStatePayload, err := proto.Marshal(toSharedPlayerState(newPlayer))
	if err != nil {
		return fmt.Errorf("failed to marshal welcome: %w", err)
	}
	if err := s.broker.Send(client.ID(), protocol.MsgWelcome, newPlayerStatePayload); err != nil {
		return fmt.Errorf("failed to send welcome: %w", err)
	}

	// 現在の他プレイヤーの状態をそのクライアントに送信する
	for playerID, player := range s.game.GetPlayers() {
		if playerID == game.PlayerID(client.ID()) {
			continue
		}

		payload, err := proto.Marshal(toSharedPlayerState(player))
		if err != nil {
			return fmt.Errorf("failed to marshal player state: %w", err)
		}

		if err := s.broker.Send(client.ID(), protocol.MsgPlayerState, payload); err != nil {
			return fmt.Errorf("failed to send player state: %w", err)
		}
	}

	// 現在のアイテムの状態をそのクライアントに送信する
	for _, item := range s.game.GetItems() {
		payload, err := proto.Marshal(toActiveSharedItemState(item))
		if err != nil {
			return fmt.Errorf("failed to marshal item state: %w", err)
		}

		if err := s.broker.Send(client.ID(), protocol.MsgItemState, payload); err != nil {
			return fmt.Errorf("failed to send item state: %w", err)
		}
	}

	// 新規プレイヤーの参加を全クライアントに配信する
	if err := s.broker.Broadcast(protocol.MsgPlayerState, newPlayerStatePayload); err != nil {
		return fmt.Errorf("failed to broadcast new player state: %w", err)
	}

	return nil
}

// OnMessage はメッセージ種別に応じて適切なハンドラに振り分ける
func (s *GameService) OnMessage(client *Client, msg protocol.Message) error {
	switch msg.Type {
	case protocol.MsgPlayerState:
		return s.onReceivePlayerState(client, msg.Payload)
	case protocol.MsgPlayerAction:
		return s.onReceivePlayerAction(client, msg.Payload)
	default:
		return fmt.Errorf("unknown message type: 0x%02x", msg.Type)
	}
}

// OnDisconnected はクライアントをBrokerとGameから削除し、切断を全員に通知する
func (s *GameService) OnDisconnected(client *Client) error {
	s.broker.RemoveClient(client)
	s.game.RemovePlayer(game.PlayerID(client.ID()))

	playerState := &shared.PlayerState{
		PlayerId: client.ID(),
		Status:   shared.Status_DISCONNECTED,
	}
	payload, err := proto.Marshal(playerState)
	if err != nil {
		return fmt.Errorf("failed to marshal player state: %w", err)
	}
	if err := s.broker.Broadcast(protocol.MsgPlayerState, payload); err != nil {
		return fmt.Errorf("failed to broadcast player state: %w", err)
	}

	return nil
}

// onReceivePlayerState はクライアントから受け取ったプレイヤーの位置・向きをゲームに反映し、全クライアントに配信する
func (s *GameService) onReceivePlayerState(client *Client, payload []byte) error {
	playerID := game.PlayerID(client.ID())
	playerState := &shared.PlayerState{}
	if err := proto.Unmarshal(payload, playerState); err != nil {
		return fmt.Errorf("failed to unmarshal player state: %w", err)
	}

	direction, err := fromSharedDirection(playerState.GetDirection())
	if err != nil {
		// 方向が不正な場合は無視する
		return nil
	}

	updatedPlayer := s.game.MovePlayer(
		playerID,
		game.Position{
			X: int(playerState.GetPosition().GetX()),
			Y: int(playerState.GetPosition().GetY()),
		},
		direction,
	)

	broadcastPayload, err := proto.Marshal(toSharedPlayerState(updatedPlayer))
	if err != nil {
		return fmt.Errorf("failed to marshal player state: %w", err)
	}
	if err := s.broker.Broadcast(protocol.MsgPlayerState, broadcastPayload); err != nil {
		return fmt.Errorf("failed to broadcast player state: %w", err)
	}

	return nil
}

// onReceivePlayerAction はクライアントからのアクションを処理する（弾の発射など）
func (s *GameService) onReceivePlayerAction(client *Client, payload []byte) error {
	playerID := game.PlayerID(client.ID())

	playerActionRequest := &shared.PlayerActionRequest{}
	if err := proto.Unmarshal(payload, playerActionRequest); err != nil {
		return fmt.Errorf("failed to unmarshal player action request: %w", err)
	}

	switch playerActionRequest.GetType() {
	case shared.ActionType_SHOOT_BULLET:
		s.game.ShootBullet(playerID)
	}

	return nil
}

// StartPublishLoop はゲームループで更新された状態を検知し、クライアントに配信するループを開始する
func (s *GameService) StartPublishLoop(ctx context.Context, updatedCh <-chan game.UpdatedResult) {
	go func() {
		for {
			select {
			case updatedResult, ok := <-updatedCh:
				if !ok {
					return
				}
				s.publishStates(updatedResult)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *GameService) publishStates(updatedResult game.UpdatedResult) {
	switch updatedResult.Type {
	case game.UpdatedResultTypeItemsUpdated:
		s.publishItemStates()
	case game.UpdatedResultTypePlayersUpdated:
		s.publishPlayerStates()
	}
}

// publishItemStates は全アイテムの現在の状態をクライアントに配信する
func (s *GameService) publishItemStates() {
	// Activeなアイテムを送信する
	for _, item := range s.game.GetItems() {
		payload, err := proto.Marshal(toActiveSharedItemState(item))
		if err != nil {
			continue
		}
		s.broker.Broadcast(protocol.MsgItemState, payload)
	}

	// 削除されたアイテムを送信する
	for _, removedItem := range s.game.GetRemovedItems() {
		itemState := &shared.ItemState{
			ItemId: string(removedItem.ID()),
			Status: shared.ItemStatus_REMOVED,
		}

		payload, err := proto.Marshal(itemState)
		if err != nil {
			continue
		}
		if err := s.broker.Broadcast(protocol.MsgItemState, payload); err != nil {
			continue
		}

		// Broadcastが成功したら削除アイテムは不要になる
		s.game.ClearRemovedItem(removedItem.ID())
	}
}

// publishPlayerStates は全プレイヤーの現在の状態をクライアントに配信する
func (s *GameService) publishPlayerStates() {
	for _, player := range s.game.GetPlayers() {
		payload, err := proto.Marshal(toSharedPlayerState(player))
		if err != nil {
			continue
		}
		s.broker.Broadcast(protocol.MsgPlayerState, payload)
	}
}

func toSharedPlayerState(player *game.Player) *shared.PlayerState {
	var status shared.Status
	switch player.Status() {
	case game.PlayerStatusAlive:
		status = shared.Status_ALIVE
	case game.PlayerStatusDead:
		status = shared.Status_DEAD
	default:
		panic(fmt.Sprintf("invalid player status: %s", player.Status()))
	}

	return &shared.PlayerState{
		PlayerId: string(player.PlayerID),
		Position: &shared.Position{
			X: int32(player.Position().X),
			Y: int32(player.Position().Y),
		},
		Direction: toSharedDirection(player.Direction()),
		Status:    status,
	}
}

func toActiveSharedItemState(item game.Item) *shared.ItemState {
	var itemType shared.ItemType
	switch item.Type() {
	case game.ItemTypeBullet:
		itemType = shared.ItemType_BULLET
	default:
		panic(fmt.Sprintf("invalid item type: %s", item.Type()))
	}

	return &shared.ItemState{
		ItemId: string(item.ID()),
		Type:   itemType,
		Position: &shared.Position{
			X: int32(item.Position().X),
			Y: int32(item.Position().Y),
		},
		Status: shared.ItemStatus_ACTIVE,
	}
}

func toSharedDirection(d game.Direction) shared.Direction {
	switch d {
	case game.DirectionUp:
		return shared.Direction_UP
	case game.DirectionDown:
		return shared.Direction_DOWN
	case game.DirectionLeft:
		return shared.Direction_LEFT
	case game.DirectionRight:
		return shared.Direction_RIGHT
	default:
		panic(fmt.Sprintf("invalid direction: %s", d))
	}
}

func fromSharedDirection(d shared.Direction) (game.Direction, error) {
	switch d {
	case shared.Direction_UP:
		return game.DirectionUp, nil
	case shared.Direction_DOWN:
		return game.DirectionDown, nil
	case shared.Direction_LEFT:
		return game.DirectionLeft, nil
	case shared.Direction_RIGHT:
		return game.DirectionRight, nil
	default:
		return "", fmt.Errorf("invalid direction: %d", d)
	}
}
