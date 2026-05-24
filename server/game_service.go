package main

import (
	"fmt"
	"log/slog"

	"google.golang.org/protobuf/proto"
	"realtime-communication-server/server/game"
	"realtime-communication-server/shared"
	"realtime-communication-server/shared/protocol"
)

// GameService はクライアントからのメッセージをゲームの状態に反映し、
// さらに他のクライアントに状態同期をする役割を持つ
type GameService struct {
	broker *Broker
	game   *game.Game
}

var _ Handler = (*GameService)(nil)

func NewGameService(broker *Broker, game *game.Game) *GameService {
	return &GameService{broker: broker, game: game}
}

// OnConnected はプレイヤーをBrokerとGameに登録し、welcomeを送り、
// 既存プレイヤーの状態を新規接続者に同期、新規プレイヤーの存在を全員に通知する。
func (s *GameService) OnConnected(client *Client) error {
	slog.Info("client connected", "playerId", client.ID())
	s.broker.AddClient(client)
	newPlayer := s.game.AddPlayer(game.PlayerID(client.ID()))

	// IDや初期位置、マップサイズなどを本人にwelcomeメッセージとして送信する
	newPlayerState := toSharedPlayerState(newPlayer)
	welcomePayload, err := proto.Marshal(&shared.Welcome{
		PlayerState: newPlayerState,
		MapWidth:    int32(s.game.Width),
		MapHeight:   int32(s.game.Height),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal welcome: %w", err)
	}
	if err := s.broker.Send(client.ID(), protocol.MsgWelcome, welcomePayload); err != nil {
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

	// 新規プレイヤーの参加を全クライアントに配信する
	newPlayerStatePayload, err := proto.Marshal(newPlayerState)
	if err != nil {
		return fmt.Errorf("failed to marshal new player state: %w", err)
	}
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
	default:
		return fmt.Errorf("unknown message type: 0x%02x", msg.Type)
	}
}

// onReceivePlayerState はクライアントから受け取ったプレイヤーの位置・向きをゲームに反映し、全クライアントに配信する
func (s *GameService) onReceivePlayerState(client *Client, payload []byte) error {
	playerID := game.PlayerID(client.ID())
	playerState := &shared.PlayerState{}
	if err := proto.Unmarshal(payload, playerState); err != nil {
		return fmt.Errorf("failed to unmarshal player state: %w", err)
	}

	slog.Info("received player state",
		"playerId", client.ID(),
		"position", playerState.GetPosition(),
		"direction", playerState.GetDirection())

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
	if updatedPlayer == nil {
		return nil
	}

	broadcastPayload, err := proto.Marshal(toSharedPlayerState(updatedPlayer))
	if err != nil {
		return fmt.Errorf("failed to marshal player state: %w", err)
	}
	if err := s.broker.Broadcast(protocol.MsgPlayerState, broadcastPayload); err != nil {
		return fmt.Errorf("failed to broadcast player state: %w", err)
	}

	return nil
}

// OnDisconnected はクライアントをBrokerとGameから削除し、切断を全員に通知する
func (s *GameService) OnDisconnected(client *Client) error {
	slog.Info("client disconnected", "playerId", client.ID())
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

func toSharedPlayerState(player *game.Player) *shared.PlayerState {
	return &shared.PlayerState{
		PlayerId: string(player.PlayerID),
		Position: &shared.Position{
			X: int32(player.Position().X),
			Y: int32(player.Position().Y),
		},
		Direction: toSharedDirection(player.Direction()),
		Status:    shared.Status_ALIVE,
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
