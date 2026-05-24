package main

import (
	"errors"
	"fmt"
	"sync"
)

// Broker は接続中の全クライアントを管理し、メッセージの配信を行う
type Broker struct {
	clients map[string]*Client
	mu      sync.RWMutex
}

func NewBroker() *Broker {
	return &Broker{
		clients: make(map[string]*Client),
	}
}

func (b *Broker) AddClient(client *Client) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clients[client.ID()] = client
}

func (b *Broker) RemoveClient(client *Client) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, client.ID())
}

// Broadcast はクライアント全員にメッセージを配信する
func (b *Broker) Broadcast(msgType byte, payload []byte) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	errs := make([]error, 0, len(b.clients))
	for _, client := range b.clients {
		if err := client.Send(msgType, payload); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Send は特定のクライアントにメッセージを送信する
func (b *Broker) Send(clientID string, msgType byte, payload []byte) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	client, ok := b.clients[clientID]
	if !ok {
		return fmt.Errorf("client not found: %s", clientID)
	}
	return client.Send(msgType, payload)
}
