package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	"google.golang.org/protobuf/proto"
	"realtime-communication-server/shared"
	"realtime-communication-server/shared/protocol"
)

type Player struct {
	Position  Position
	Direction shared.Direction
}

type Position struct {
	X int
	Y int
}

type Item struct {
	ID       string
	Type     shared.ItemType
	Position Position
}

type Game struct {
	conn net.Conn

	screen tcell.Screen

	width  int
	height int

	myPlayerID string
	players    map[string]Player
	items      map[string]Item
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("failed to create new screen: %w", err)
	}
	defer screen.Fini()

	if err := screen.Init(); err != nil {
		return fmt.Errorf("failed to initialize screen: %w", err)
	}

	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.Close()

	// サーバーがwelcomeとして自プレイヤーのIDと初期位置、マップサイズなどの初期化情報を送ってくる。
	// 自プレイヤーの初期化はwelcomeを受け取ってから行う。
	msg, err := protocol.ReadMessage(conn)
	if err != nil {
		return fmt.Errorf("failed to read welcome: %w", err)
	}
	if msg.Type != protocol.MsgWelcome {
		return fmt.Errorf("expected welcome message, got 0x%02x", msg.Type)
	}
	welcome := &shared.Welcome{}
	if err := proto.Unmarshal(msg.Payload, welcome); err != nil {
		return fmt.Errorf("failed to unmarshal welcome: %w", err)
	}
	myPlayerState := welcome.GetPlayerState()

	game := &Game{
		conn: conn,

		screen: screen,

		width:  int(welcome.GetMapWidth()),
		height: int(welcome.GetMapHeight()),

		myPlayerID: myPlayerState.GetPlayerId(),
		players:    make(map[string]Player),
		items:      make(map[string]Item),
	}
	game.players[game.myPlayerID] = Player{
		Position: Position{
			X: int(myPlayerState.GetPosition().GetX()),
			Y: int(myPlayerState.GetPosition().GetY()),
		},
		Direction: myPlayerState.GetDirection(),
	}

	// サーバーからのメッセージを別goroutineで受信し続ける
	messageChan := make(chan protocol.Message)
	go func() {
		for {
			msg, err := protocol.ReadMessage(conn)
			if err != nil {
				close(messageChan)
				return
			}
			messageChan <- msg
		}
	}()

	// メインループ: キー入力・サーバーメッセージ・描画タイマーの3つのイベントを処理する
	ticker := time.NewTicker(time.Second / 60)
	for {
		select {
		case event := <-screen.EventQ():
			if game.handleEvent(event) {
				return nil
			}
		case msg, ok := <-messageChan:
			if !ok {
				return fmt.Errorf("server connection closed")
			}
			game.handleMessage(msg)
		case <-ticker.C:
			game.draw()
		}
	}
}

// publishMyState は自分のプレイヤー状態をサーバーに送信する。
// サーバーはこの状態を受け取り、他の全クライアントに配信する。
func (g *Game) publishMyState() {
	myPlayer := g.getMyPlayer()

	state := &shared.PlayerState{
		PlayerId: g.myPlayerID,
		Position: &shared.Position{
			X: int32(myPlayer.Position.X),
			Y: int32(myPlayer.Position.Y),
		},
		Direction: myPlayer.Direction,
	}

	data, err := proto.Marshal(state)
	if err != nil {
		return
	}

	protocol.WriteMessage(g.conn, protocol.Message{
		Type:    protocol.MsgPlayerState,
		Payload: data,
	})
}

// handleMessage はサーバーから受信したメッセージを処理する。
// 他プレイヤーの状態変化（移動・切断）を players map に反映する。
func (g *Game) handleMessage(msg protocol.Message) {
	switch msg.Type {
	case protocol.MsgPlayerState:
		playerState := &shared.PlayerState{}
		if err := proto.Unmarshal(msg.Payload, playerState); err != nil {
			return
		}

		if playerState.GetStatus() == shared.Status_DISCONNECTED {
			delete(g.players, playerState.GetPlayerId())
			return
		}

		g.players[playerState.GetPlayerId()] = Player{
			Position: Position{
				X: int(playerState.GetPosition().GetX()),
				Y: int(playerState.GetPosition().GetY()),
			},
			Direction: playerState.GetDirection(),
		}
	case protocol.MsgItemState:
		// 弾などのアイテム状態が配信されてくる（アイテムの移動や状態はサーバーが計算する）
		itemState := &shared.ItemState{}
		if err := proto.Unmarshal(msg.Payload, itemState); err != nil {
			return
		}

		if itemState.GetStatus() == shared.ItemStatus_REMOVED {
			delete(g.items, itemState.GetItemId())
			return
		}

		g.items[itemState.GetItemId()] = Item{
			ID:   itemState.GetItemId(),
			Type: itemState.GetType(),
			Position: Position{
				X: int(itemState.GetPosition().GetX()),
				Y: int(itemState.GetPosition().GetY()),
			},
		}
	}
}

// handleEvent はキー入力イベントを処理する。終了操作の場合はtrueを返す。
func (g *Game) handleEvent(event tcell.Event) bool {
	switch ev := event.(type) {
	case *tcell.EventKey:
		switch ev.Key() {
		case tcell.KeyEscape, tcell.KeyCtrlC:
			return true
		case tcell.KeyLeft:
			g.movePlayer(shared.Direction_LEFT)
		case tcell.KeyRight:
			g.movePlayer(shared.Direction_RIGHT)
		case tcell.KeyUp:
			g.movePlayer(shared.Direction_UP)
		case tcell.KeyDown:
			g.movePlayer(shared.Direction_DOWN)
		case tcell.KeyRune:
			if ev.Str() == " " {
				g.shootBullet()
			}
		}
	}
	return false
}

// movePlayer は自プレイヤーを指定方向に移動させ、変更があればサーバーに送信する。
func (g *Game) movePlayer(direction shared.Direction) {
	myPlayer := g.getMyPlayer()
	oldX, oldY := myPlayer.Position.X, myPlayer.Position.Y
	oldDirection := myPlayer.Direction

	var dx, dy int
	switch direction {
	case shared.Direction_LEFT:
		dx = -1
	case shared.Direction_RIGHT:
		dx = 1
	case shared.Direction_UP:
		dy = -1
	case shared.Direction_DOWN:
		dy = 1
	}

	if newX := myPlayer.Position.X + dx; newX >= 0 && newX < g.width {
		myPlayer.Position.X = newX
	}
	if newY := myPlayer.Position.Y + dy; newY >= 0 && newY < g.height {
		myPlayer.Position.Y = newY
	}
	myPlayer.Direction = direction

	g.players[g.myPlayerID] = myPlayer

	// 位置か方向が変更された場合のみサーバーに送信する
	if oldX != myPlayer.Position.X || oldY != myPlayer.Position.Y || oldDirection != direction {
		g.publishMyState()
	}
}

// shootBullet は弾発射アクションをサーバーに送信する。
// クライアントは「撃ちたい」というリクエストを送るだけで、
// 弾の生成や移動計算はサーバーが行う。
func (g *Game) shootBullet() {
	req := &shared.PlayerActionRequest{
		Type: shared.ActionType_SHOOT_BULLET,
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return
	}

	protocol.WriteMessage(g.conn, protocol.Message{
		Type:    protocol.MsgPlayerAction,
		Payload: data,
	})
}

func (g *Game) getMyPlayer() Player {
	return g.players[g.myPlayerID]
}

// draw はゲーム画面を描画する。
func (g *Game) draw() {
	g.screen.Clear()

	style := tcell.StyleDefault.
		Background(color.White).
		Foreground(color.Black)

	// マップを描画
	for y := range g.height {
		for x := range g.width {
			g.screen.SetContent(x, y, '.', nil, style)
		}
	}

	// プレイヤーを描画
	for id, player := range g.players {
		g.screen.SetContent(
			player.Position.X,
			player.Position.Y,
			getPlayerRune(player, id == g.myPlayerID),
			nil,
			style,
		)
	}

	// アイテムを描画
	for _, item := range g.items {
		switch item.Type {
		case shared.ItemType_BULLET:
			g.screen.SetContent(
				item.Position.X,
				item.Position.Y,
				'*',
				nil,
				style,
			)
		default:
			// 知らないアイテムは描画しない
		}
	}

	g.screen.Show()
}

var myPlayerRunes = map[shared.Direction]rune{
	shared.Direction_UP:    '▲',
	shared.Direction_DOWN:  '▼',
	shared.Direction_LEFT:  '◀',
	shared.Direction_RIGHT: '▶',
}

var otherPlayerRunes = map[shared.Direction]rune{
	shared.Direction_UP:    '^',
	shared.Direction_DOWN:  'v',
	shared.Direction_LEFT:  '<',
	shared.Direction_RIGHT: '>',
}

func getPlayerRune(player Player, isMyPlayer bool) rune {
	runes := otherPlayerRunes
	if isMyPlayer {
		runes = myPlayerRunes
	}
	return runes[player.Direction]
}
