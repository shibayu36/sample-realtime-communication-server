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

// Handler はクライアントの接続・メッセージ受信・切断のイベントを処理する
type Handler interface {
	// OnConnected はクライアントが接続した時に呼ばれる
	OnConnected(client *Client) error
	// OnMessage はクライアントからメッセージを受信した時に呼ばれる
	OnMessage(client *Client, msg protocol.Message) error
	// OnDisconnected はクライアントが切断した時に呼ばれる
	OnDisconnected(client *Client) error
}

// Server はクライアントからのTCP接続を受け付け、接続・メッセージ受信・切断のイベントをHandlerに委譲する
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

// handleConnection はクライアント1つの接続から切断までを処理する
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	// 最初のメッセージからクライアントIDを取得し、Handlerに接続を通知する
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

	// clientID取得に使った最初のメッセージも、通常のメッセージとして処理する
	s.handler.OnMessage(client, firstMsg)

	// クライアントが切断するまでメッセージを読み続ける
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
