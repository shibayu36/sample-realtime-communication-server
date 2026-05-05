package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	// MsgWelcome は接続直後にサーバーがクライアントへ1度だけ送り、自プレイヤーのIDや初期状態、ゲームシステム情報を通知する。
	// payload: shared.Welcome
	MsgWelcome byte = 0x01
	// MsgPlayerState はプレイヤーの状態を通知する。
	// クライアントは自分の状態変化時にサーバーへ送信し、サーバーは全クライアントへ配信する（双方向）。
	// payload: shared.PlayerState
	MsgPlayerState byte = 0x02
	// MsgPlayerAction はクライアントからサーバーへ弾発射などのアクションを要求する。
	// payload: shared.PlayerActionRequest
	MsgPlayerAction byte = 0x03
	// MsgItemState はアイテム（弾など）の状態をサーバーから全クライアントへ配信する。
	// payload: shared.ItemState
	MsgItemState byte = 0x04

	headerSize     = 5           // 1(type) + 4(length)
	maxPayloadSize = 1024 * 1024 // 1MB上限
)

// Message はワイヤー上の1メッセージを表す
type Message struct {
	Type    byte
	Payload []byte
}

// WriteMessage は conn にメッセージを書き込む
func WriteMessage(w io.Writer, msg Message) error {
	header := make([]byte, headerSize)
	header[0] = msg.Type
	binary.BigEndian.PutUint32(header[1:], uint32(len(msg.Payload)))

	if _, err := w.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if _, err := w.Write(msg.Payload); err != nil {
		return fmt.Errorf("failed to write payload: %w", err)
	}
	return nil
}

// ReadMessage は conn から1メッセージを読み取る
// TCPはストリームなので Read だと途中までしか読めない場合がある。
// io.ReadFull は指定バイト数きっちり読むまで待つ。
func ReadMessage(r io.Reader) (Message, error) {
	header := make([]byte, headerSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return Message{}, fmt.Errorf("failed to read header: %w", err)
	}

	msgType := header[0]
	length := binary.BigEndian.Uint32(header[1:])

	if length > maxPayloadSize {
		return Message{}, fmt.Errorf("payload too large: %d", length)
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return Message{}, fmt.Errorf("failed to read payload: %w", err)
	}

	return Message{Type: msgType, Payload: payload}, nil
}
