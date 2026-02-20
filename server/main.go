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
	g := game.NewGame(30, 30)
	service := NewGameService(broker, g)

	server, err := NewServer(":8080", service)
	if err != nil {
		return err
	}

	updatedCh := g.StartUpdateLoop(ctx)
	service.StartPublishLoop(ctx, updatedCh)

	go func() {
		<-ctx.Done()
		server.Close()
	}()

	server.Serve()
	return nil
}
