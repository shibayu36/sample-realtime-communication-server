package main

import (
	"fmt"
	"net"
	"sync"

	"realtime-communication-server/shared/protocol"
)

// Client は接続中のクライアントを表す
type Client struct {
	id   string
	conn net.Conn
	mu   sync.Mutex
}

func (c *Client) ID() string {
	return c.id
}

// Send はこのクライアントにメッセージを送信する
func (c *Client) Send(msgType byte, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := protocol.WriteMessage(c.conn, protocol.Message{
		Type:    msgType,
		Payload: payload,
	})
	if err != nil {
		return fmt.Errorf("failed to send message to client %s: %w", c.id, err)
	}
	return nil
}
