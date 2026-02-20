package main

import (
	"errors"
	"fmt"
	"sync"
)

type Broker struct {
	clients    map[string]Client
	clientsMux sync.RWMutex
}

func NewBroker() *Broker {
	return &Broker{
		clients: make(map[string]Client),
	}
}

func (b *Broker) AddClient(client Client) {
	b.clientsMux.Lock()
	defer b.clientsMux.Unlock()
	b.clients[client.ID()] = client
}

func (b *Broker) RemoveClient(client Client) {
	b.clientsMux.Lock()
	defer b.clientsMux.Unlock()
	delete(b.clients, client.ID())
}

// Broadcast クライアント全員にメッセージを配信する
func (b *Broker) Broadcast(msgType byte, payload []byte) error {
	b.clientsMux.RLock()
	defer b.clientsMux.RUnlock()

	errs := make([]error, 0, len(b.clients))
	for _, client := range b.clients {
		if err := client.Send(msgType, payload); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Send 特定のクライアントにメッセージを送信する
func (b *Broker) Send(clientID string, msgType byte, payload []byte) error {
	b.clientsMux.RLock()
	defer b.clientsMux.RUnlock()

	client, ok := b.clients[clientID]
	if !ok {
		return fmt.Errorf("client not found: %s", clientID)
	}
	return client.Send(msgType, payload)
}
