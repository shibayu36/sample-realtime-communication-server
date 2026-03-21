package main

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/shibayu36/sample-realtime-communication-server/shared"
	"github.com/shibayu36/sample-realtime-communication-server/shared/protocol"
	"google.golang.org/protobuf/proto"
)

type Handler interface {
	OnConnected(client *Client) error
	OnMessage(client *Client, msg protocol.Message) error
	OnDisconnected(client *Client) error
}

type Server struct {
	listener net.Listener
	handler  Handler
}

func NewServer(address string, handler Handler) (*Server, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	return &Server{
		listener: listener,
		handler:  handler,
	}, nil
}

func (s *Server) Close() error {
	return s.listener.Close()
}

func (s *Server) Serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// listener.Closeが呼ばれたら終了
			return
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	// 最初のメッセージからPlayerStateをunmarshalしてclientIDを取得
	firstMsg, err := protocol.ReadMessage(conn)
	if err != nil {
		return
	}

	playerState := &shared.PlayerState{}
	if err := proto.Unmarshal(firstMsg.Payload, playerState); err != nil {
		return
	}

	client := &Client{id: playerState.GetPlayerId(), conn: conn}

	if err := s.handler.OnConnected(client); err != nil {
		return
	}

	defer func() {
		s.handler.OnDisconnected(client)
	}()

	// 最初のメッセージもハンドリングする
	s.handler.OnMessage(client, firstMsg)

	// メッセージループ
	for {
		msg, err := protocol.ReadMessage(conn)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) {
				return
			}
			return
		}

		s.handler.OnMessage(client, msg)
	}
}
