package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"realtime-communication-server/server/game"
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

	// ゲームループが更新を検知し、配信ループがそれを全クライアントへ送る
	updatedCh := g.StartUpdateLoop(ctx)
	service.StartPublishLoop(ctx, updatedCh)

	server, err := NewServer(":8080", service)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		server.Close()
	}()

	slog.Info("server started", "addr", ":8080")
	server.Serve()
	return nil
}
