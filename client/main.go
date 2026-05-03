package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	"github.com/google/uuid"
)

type Player struct {
	Position  Position
	Direction Direction
}

type Position struct {
	X int
	Y int
}

type Direction int

const (
	DirectionUp Direction = iota
	DirectionDown
	DirectionLeft
	DirectionRight
)

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

	// 自プレイヤーを作成。実装を進めるとサーバー経由で初期化される
	myPlayerID := uuid.New().String()
	game := &Game{
		screen: screen,

		width:  40,
		height: 20,

		myPlayerID: myPlayerID,
		players:    make(map[string]Player),
	}
	game.players[myPlayerID] = Player{
		Position:  Position{X: rand.Intn(game.width), Y: rand.Intn(game.height)},
		Direction: DirectionUp,
	}

	// 動かないダミープレイヤー。実装を進めるとサーバー経由の他プレイヤーに置き換わる
	game.players[uuid.New().String()] = Player{
		Position:  Position{X: rand.Intn(game.width), Y: rand.Intn(game.height)},
		Direction: DirectionDown,
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
			g.movePlayer(DirectionLeft)
		case tcell.KeyRight:
			g.movePlayer(DirectionRight)
		case tcell.KeyUp:
			g.movePlayer(DirectionUp)
		case tcell.KeyDown:
			g.movePlayer(DirectionDown)
		}
	}
	return false
}

// movePlayer は自プレイヤーを指定方向に1マス移動させる。
// マップ範囲外には出ないようにする。
func (g *Game) movePlayer(direction Direction) {
	p := g.players[g.myPlayerID]

	var dx, dy int
	switch direction {
	case DirectionLeft:
		dx = -1
	case DirectionRight:
		dx = 1
	case DirectionUp:
		dy = -1
	case DirectionDown:
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

var myPlayerRunes = map[Direction]rune{
	DirectionUp:    '▲',
	DirectionDown:  '▼',
	DirectionLeft:  '◀',
	DirectionRight: '▶',
}

var otherPlayerRunes = map[Direction]rune{
	DirectionUp:    '^',
	DirectionDown:  'v',
	DirectionLeft:  '<',
	DirectionRight: '>',
}

func getPlayerRune(player Player, isMyPlayer bool) rune {
	runes := otherPlayerRunes
	if isMyPlayer {
		runes = myPlayerRunes
	}
	return runes[player.Direction]
}
