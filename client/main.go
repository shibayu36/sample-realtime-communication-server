package main

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/google/uuid"
	"github.com/shibayu36/sample-realtime-communication-server/shared"
	"github.com/shibayu36/sample-realtime-communication-server/shared/protocol"
	"google.golang.org/protobuf/proto"
)

type Position struct {
	X int
	Y int
}

type Player struct {
	ID        string
	Position  Position
	Direction shared.Direction
	Status    shared.Status
}

type Item struct {
	ID       string
	Type     shared.ItemType
	Position Position
}

type Game struct {
	conn net.Conn

	screen tcell.Screen

	myPlayerID string
	players    map[string]Player
	items      map[string]Item
	width      int
	height     int
}

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

	// 位置か方向が変更されたら自分の状態をサーバーに送る
	if oldX != myPlayer.Position.X || oldY != myPlayer.Position.Y || oldDirection != direction {
		g.publishMyState()
	}
}

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
			if ev.Rune() == ' ' {
				g.shootBullet()
			}
		}
	}
	return false
}

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

func getPlayerRune(player Player) rune {
	if player.Status == shared.Status_DEAD {
		return 'x'
	}

	switch player.Direction {
	case shared.Direction_UP:
		return '^'
	case shared.Direction_DOWN:
		return 'v'
	case shared.Direction_LEFT:
		return '<'
	case shared.Direction_RIGHT:
		return '>'
	default:
		return '^'
	}
}

const (
	bgColor          = tcell.Color232
	mapColor         = tcell.Color255
	myPlayerColor    = tcell.Color46
	otherPlayerColor = tcell.Color196
	itemColor        = tcell.Color226
)

func (g *Game) draw() {
	g.screen.Clear()

	defaultStyle := tcell.StyleDefault.
		Background(bgColor).
		Foreground(mapColor)

	// マップを描画
	for y := range g.height {
		for x := range g.width {
			g.screen.SetContent(x, y, '.', nil, defaultStyle)
		}
	}

	// プレイヤーを描画
	myPlayerStyle := defaultStyle.Foreground(myPlayerColor)
	otherPlayerStyle := defaultStyle.Foreground(otherPlayerColor)
	for _, player := range g.players {
		style := otherPlayerStyle
		if player.ID == g.myPlayerID {
			style = myPlayerStyle
		}
		g.screen.SetContent(
			player.Position.X,
			player.Position.Y,
			getPlayerRune(player),
			nil,
			style,
		)
	}

	// アイテムを描画
	itemStyle := defaultStyle.Foreground(itemColor)
	for _, item := range g.items {
		g.screen.SetContent(
			item.Position.X,
			item.Position.Y,
			'*',
			nil,
			itemStyle,
		)
	}

	g.screen.Show()
}

func (g *Game) getMyPlayer() Player {
	return g.players[g.myPlayerID]
}

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
			ID: playerState.GetPlayerId(),
			Position: Position{
				X: int(playerState.GetPosition().GetX()),
				Y: int(playerState.GetPosition().GetY()),
			},
			Direction: playerState.GetDirection(),
			Status:    playerState.GetStatus(),
		}
	case protocol.MsgItemState:
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

func run() error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("failed to create new screen: %w", err)
	}
	defer screen.Fini()

	if err := screen.Init(); err != nil {
		return fmt.Errorf("failed to initialize screen: %w", err)
	}

	// TCP接続
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.Close()

	clientID := uuid.New().String()

	game := &Game{
		conn:       conn,
		myPlayerID: clientID,
		screen:     screen,
		width:      30,
		height:     30,
		players:    make(map[string]Player),
		items:      make(map[string]Item),
	}

	// プレイヤーをwidthとheightの範囲内でランダムに配置
	game.players[clientID] = Player{
		ID:        clientID,
		Position:  Position{X: rand.Intn(game.width), Y: rand.Intn(game.height)},
		Direction: shared.Direction_UP,
		Status:    shared.Status_ALIVE,
	}

	// screenからのイベントを受け取る
	eventChan := make(chan tcell.Event)
	go func() {
		for {
			eventChan <- screen.PollEvent()
		}
	}()

	// サーバーからのメッセージ受信ループ
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

	// 自分の初期位置を送信
	game.publishMyState()

	// メインループ
	ticker := time.NewTicker(50 * time.Millisecond)
	for {
		select {
		case event := <-eventChan:
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

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
