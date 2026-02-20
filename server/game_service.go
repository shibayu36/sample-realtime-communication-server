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

func (s *GameService) OnConnected(client Client) error {
	s.broker.AddClient(client)
	s.game.AddPlayer(game.PlayerID(client.ID()))

	// 現在の他プレイヤーの位置をそのクライアントに送信する
	for playerID, player := range s.game.GetPlayers() {
		if playerID == game.PlayerID(client.ID()) {
			continue
		}

		payload, err := proto.Marshal(player.ToSharedPlayerState())
		if err != nil {
			return fmt.Errorf("failed to marshal player state: %w", err)
		}

		if err := s.broker.Send(client.ID(), protocol.MsgPlayerState, payload); err != nil {
			return fmt.Errorf("failed to send player state: %w", err)
		}
	}

	return nil
}

func (s *GameService) OnMessage(client Client, msg protocol.Message) error {
	switch msg.Type {
	case protocol.MsgPlayerState:
		return s.onReceivePlayerState(client, msg.Payload)
	case protocol.MsgPlayerAction:
		return s.onReceivePlayerAction(client, msg.Payload)
	default:
		return fmt.Errorf("unknown message type: 0x%02x", msg.Type)
	}
}

func (s *GameService) OnDisconnected(client Client) error {
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

// player_stateメッセージを受信した時の処理
func (s *GameService) onReceivePlayerState(client Client, payload []byte) error {
	playerID := game.PlayerID(client.ID())
	playerState := &shared.PlayerState{}
	if err := proto.Unmarshal(payload, playerState); err != nil {
		return fmt.Errorf("failed to unmarshal player state: %w", err)
	}

	direction, err := game.FromSharedDirection(playerState.GetDirection())
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

	broadcastPayload, err := proto.Marshal(updatedPlayer.ToSharedPlayerState())
	if err != nil {
		return fmt.Errorf("failed to marshal player state: %w", err)
	}
	if err := s.broker.Broadcast(protocol.MsgPlayerState, broadcastPayload); err != nil {
		return fmt.Errorf("failed to broadcast player state: %w", err)
	}

	return nil
}

func (s *GameService) onReceivePlayerAction(client Client, payload []byte) error {
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

// StartPublishLoop ゲームの状態を定期的にpublishするループを開始する
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

func (s *GameService) publishItemStates() {
	// Activeなアイテムを送信する
	for _, item := range s.game.GetItems() {
		itemState := &shared.ItemState{
			ItemId: string(item.ID()),
			Type:   item.Type().ToSharedItemType(),
			Position: &shared.Position{
				X: int32(item.Position().X),
				Y: int32(item.Position().Y),
			},
			Status: shared.ItemStatus_ACTIVE,
		}

		payload, err := proto.Marshal(itemState)
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

func (s *GameService) publishPlayerStates() {
	for _, player := range s.game.GetPlayers() {
		payload, err := proto.Marshal(player.ToSharedPlayerState())
		if err != nil {
			continue
		}
		s.broker.Broadcast(protocol.MsgPlayerState, payload)
	}
}
