package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
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
	service := NewGameService(broker)

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
