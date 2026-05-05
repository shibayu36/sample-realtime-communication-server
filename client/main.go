package main

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	"github.com/google/uuid"
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

type Game struct {
	screen tcell.Screen

	width  int
	height int

	myPlayerID string
	players    map[string]Player
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
		screen: screen,

		width:  int(welcome.GetMapWidth()),
		height: int(welcome.GetMapHeight()),

		myPlayerID: myPlayerState.GetPlayerId(),
		players:    make(map[string]Player),
	}
	game.players[game.myPlayerID] = Player{
		Position: Position{
			X: int(myPlayerState.GetPosition().GetX()),
			Y: int(myPlayerState.GetPosition().GetY()),
		},
		Direction: myPlayerState.GetDirection(),
	}

	// 動かないダミープレイヤー。実装を進めるとサーバー経由の他プレイヤーに置き換わる
	game.players[uuid.New().String()] = Player{
		Position:  Position{X: rand.Intn(game.width), Y: rand.Intn(game.height)},
		Direction: shared.Direction_DOWN,
	}

	// メインループ: キー入力と描画タイマーの2つを処理する
	ticker := time.NewTicker(time.Second / 60)
	for {
		select {
		case event := <-screen.EventQ():
			if game.handleEvent(event) {
				return nil
			}
		case <-ticker.C:
			game.draw()
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
		}
	}
	return false
}

// movePlayer は自プレイヤーを指定方向に1マス移動させる。
// マップ範囲外には出ないようにする。
func (g *Game) movePlayer(direction shared.Direction) {
	p := g.players[g.myPlayerID]

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

	if newX := p.Position.X + dx; newX >= 0 && newX < g.width {
		p.Position.X = newX
	}
	if newY := p.Position.Y + dy; newY >= 0 && newY < g.height {
		p.Position.Y = newY
	}
	p.Direction = direction

	g.players[g.myPlayerID] = p
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
