package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/shibayu36/sample-realtime-communication-server/server/game"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	broker := NewBroker()
	g := game.NewGame(40, 20) // 40x20のゲーム空間を作成

	service := NewGameService(broker, g)

	// GameServiceはServerのHandlerとして、クライアントからのメッセージをGameに反映
	server, err := NewServer(":8080", service)
	if err != nil {
		return err
	}

	// Gameの状態更新ループを開始し、更新があればGameServiceがクライアントに配信
	updatedCh := g.StartUpdateLoop(ctx)
	service.StartPublishLoop(ctx, updatedCh)

	go func() {
		<-ctx.Done()
		server.Close()
	}()

	// クライアントからの接続を受け付ける
	server.Serve()

	return nil
}
