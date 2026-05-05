package main

import (
	"fmt"
	"log/slog"
	"math/rand"

	"google.golang.org/protobuf/proto"
	"realtime-communication-server/shared"
	"realtime-communication-server/shared/protocol"
)

// GameService はクライアントからのメッセージをゲームの状態に反映し、
// さらに他のクライアントに状態同期をする役割を持つ
type GameService struct {
	broker *Broker
}

var _ Handler = (*GameService)(nil)

func NewGameService(broker *Broker) *GameService {
	return &GameService{broker: broker}
}

func (s *GameService) OnConnected(client *Client) error {
	slog.Info("client connected", "playerId", client.ID())
	s.broker.AddClient(client)

	// IDや初期位置、マップサイズなどを本人にwelcomeメッセージとして送信する
	welcomePayload, err := proto.Marshal(&shared.Welcome{
		PlayerState: &shared.PlayerState{
			PlayerId:  client.ID(),
			Position:  &shared.Position{X: int32(rand.Intn(40)), Y: int32(rand.Intn(20))},
			Direction: shared.Direction_UP,
		},
		MapWidth:  40,
		MapHeight: 20,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal welcome: %w", err)
	}
	if err := s.broker.Send(client.ID(), protocol.MsgWelcome, welcomePayload); err != nil {
		return fmt.Errorf("failed to send welcome: %w", err)
	}
	return nil
}

func (s *GameService) OnMessage(client *Client, msg protocol.Message) error {
	// 次章以降でクライアントからのメッセージを処理する
	return nil
}

func (s *GameService) OnDisconnected(client *Client) error {
	slog.Info("client disconnected", "playerId", client.ID())
	s.broker.RemoveClient(client)
	// 次章以降で他クライアントへの切断通知やプレイヤー削除を行う
	return nil
}
