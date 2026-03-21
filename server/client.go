package main

import (
	"fmt"
	"net"
	"sync"

	"github.com/shibayu36/sample-realtime-communication-server/shared/protocol"
)

// Client は接続中のクライアントを表す
type Client interface {
	ID() string
	Send(msgType byte, payload []byte) error
}

type client struct {
	id      string
	conn    net.Conn
	sendMux sync.Mutex
}

var _ Client = (*client)(nil)

func (c *client) ID() string {
	return c.id
}

// Send はこのクライアントにメッセージを送信する
func (c *client) Send(msgType byte, payload []byte) error {
	c.sendMux.Lock()
	defer c.sendMux.Unlock()

	err := protocol.WriteMessage(c.conn, protocol.Message{
		Type:    msgType,
		Payload: payload,
	})
	if err != nil {
		return fmt.Errorf("failed to send message to client %s: %w", c.id, err)
	}
	return nil
}
