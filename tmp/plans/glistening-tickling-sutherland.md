# PoC実装計画: サンプルリアルタイム通信サーバー

## Context

書籍「ターミナルゲームで学ぶリアルタイム通信サーバー」のサンプルコード最終形（Ch5完了時点）をPoCとして一気に実装する。terminal-shooter（MQTTベース）を参考に、book-planの設計方針に沿ってTCP+独自フレーミング+protobufに再実装する。

**目的**: 最終形のコードを先に完成させ、全体像を把握した上で、後から章ごとのcommit分割を行う。

**terminal-shooterからの主な変更点**:
- MQTT → TCP + 独自プロトコル（`[1byte:種別][4bytes:長さ][N bytes:protobuf]`）
- cockroachdb/errors → 標準 `fmt.Errorf` + `errors.Join`
- Prometheus/stats削除、ログ削除、テスト不要
- ボム（bomb, bomb_fire）削除 — Ch5スコープは弾+衝突判定まで
- **名前変更**: Hooker → `Handler`, Controller → `GameService` (game_service.go)
- **ctx伝搬**: goroutineの安全な停止のためにcontext.Contextを伝搬する（簡易版。activeConn管理やWaitGroupは不要）

---

## ディレクトリ構造

```
sample-realtime-communication-server/
├── go.mod
├── Makefile
├── shared/
│   ├── proto/
│   │   └── game.proto         # Protocol Buffers定義
│   ├── game.pb.go             # 生成コード
│   ├── protocol/
│   │   └── protocol.go        # ワイヤープロトコル（~50行）
│   └── util.go                # CopyMap
├── server/
│   ├── main.go                # エントリーポイント
│   ├── server.go              # TCPサーバー + Handler interface
│   ├── client.go              # Client interface & 実装
│   ├── broker.go              # ブロードキャスト
│   ├── game_service.go        # GameService（Handler実装、メッセージ⇔ゲーム橋渡し）
│   └── game/
│       ├── primitive.go       # 基本型（Position, Direction, PlayerID, ItemID）
│       ├── collision.go       # collidable interface, collision struct
│       ├── item.go            # Item interface, ItemType
│       ├── bullet.go          # Bullet実装
│       ├── player.go          # Player実装
│       └── game.go            # Game状態管理 & 更新ループ
└── client/
    └── main.go                # TUIクライアント
```

---

## 参考にするterminal-shooterのファイル

| PoCファイル | 参考元 (terminal-shooter) |
|---|---|
| shared/protocol/protocol.go | 新規（book-planのサンプルコード） |
| shared/proto/game.proto | `shared/proto/game.proto` — ボム関連enum削除 |
| shared/util.go | `shared/util.go` — そのまま |
| server/server.go | `server/server.go` — MQTTハンドシェイク全削除、Handler interface簡素化 |
| server/client.go | `server/client.go` — Publish→Send変更 |
| server/broker.go | `server/broker.go` — topic→msgType変更 |
| server/game_service.go | `server/controller.go` — OnSubscribed統合、topic→msgType |
| server/game/*.go | `server/game/*.go` — ボム関連削除 |
| client/main.go | `client/main.go` — MQTT→TCP |

---

## 各ファイルの設計

### 1. `go.mod`
```
module github.com/shibayu36/sample-realtime-communication-server
```
依存: `github.com/gdamore/tcell/v2`, `github.com/google/uuid`, `google.golang.org/protobuf`

### 2. `shared/proto/game.proto`
terminal-shooterから **BOMB, BOMB_FIRE, PLACE_BOMB を削除**。フィールド番号はそのまま維持。
- メッセージ: Position, PlayerState, ItemState, PlayerActionRequest
- enum: Direction, Status(ALIVE/DEAD/DISCONNECTED), ItemType(BULLETのみ), ItemStatus, ActionType(SHOOT_BULLETのみ)

### 3. `shared/protocol/protocol.go` (~50行) ★新規
- 定数: `MsgPlayerState(0x01)`, `MsgPlayerAction(0x02)`, `MsgItemState(0x03)`, `headerSize=5`, `maxPayloadSize=1MB`
- `Message` struct: `Type byte`, `Payload []byte`
- `WriteMessage(w io.Writer, msg Message) error` — ヘッダ+ペイロード書き込み
- `ReadMessage(r io.Reader) (Message, error)` — `io.ReadFull`でヘッダ→ペイロード読み取り

### 4. `shared/util.go` (~10行)
- `CopyMap[K,V]` — terminal-shooterと同一。game.goのGetPlayers/GetItems/GetRemovedItemsで使用

### 5. `server/main.go` (~20行)
```go
func main() {
    ctx, cancel := signal.NotifyContext(context.Background(),
        syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    broker := NewBroker()
    g := game.NewGame(30, 30)
    service := NewGameService(broker, g)
    server, err := NewServer(":8080", service)
    if err != nil { ... }

    updatedCh := g.StartUpdateLoop(ctx)
    service.StartPublishLoop(ctx, updatedCh)

    go func() {
        <-ctx.Done()
        server.Close()  // Acceptループを止める
    }()
    server.Serve()
}
```

### 6. `server/server.go` (~65行) ★MQTT→TCPの最大変更点
- `Handler` interface（旧Hooker）: **4メソッド→3メソッド** (OnSubscribed削除)
  ```go
  type Handler interface {
      OnConnected(client Client) error
      OnMessage(client Client, msg protocol.Message) error
      OnDisconnected(client Client) error
  }
  ```
- `Server` struct: `listener net.Listener`, `handler Handler`
- `NewServer`: net.Listenしてサーバー返却
- `Close`: `s.listener.Close()` でServeを停止
- `Serve`: Acceptループ → `go handleConnection(conn)`
- `handleConnection`:
  1. 最初のメッセージからPlayerStateをunmarshalしてclientIDを取得
  2. `handler.OnConnected`呼び出し
  3. 最初のメッセージを`handler.OnMessage`で処理
  4. メッセージループ（`protocol.ReadMessage` → `handler.OnMessage`）
  5. EOF or error → `handler.OnDisconnected` + `conn.Close()`

### 7. `server/client.go` (~25行)
- `Client` interface: `ID() string`, `Send(msgType byte, payload []byte) error`
- `client` struct: `id string`, `conn net.Conn`, `sendMux sync.Mutex`
- `Send`: `sendMux.Lock` → `protocol.WriteMessage(conn, msg)`

### 8. `server/broker.go` (~45行)
- `Broadcast(msgType byte, payload []byte) error` — topic string → msgType byte
  - 各clientの`Send`を呼ぶだけ
- `Send(clientID, msgType, payload)` — 特定クライアント送信
- エラー結合: `errors.Join`（標準ライブラリ）

### 9. `server/game_service.go` (~130行) ★旧controller.go
- `GameService` struct: `broker *Broker`, `game *game.Game`
- `var _ Handler = (*GameService)(nil)`
- `OnConnected`: broker.AddClient + game.AddPlayer + **既存プレイヤー情報送信**（OnSubscribedから移動）
- `OnMessage`: `msg.Type`でswitch
  - `MsgPlayerState` → `onReceivePlayerState`
  - `MsgPlayerAction` → `onReceivePlayerAction`
- `OnDisconnected`: broker.RemoveClient + game.RemovePlayer + DISCONNECTED broadcast
- `onReceivePlayerState`: unmarshal → game.MovePlayer → broadcast
- `onReceivePlayerAction`: unmarshal → ShootBullet（PlaceBomb分岐削除）
- `StartPublishLoop(ctx, updatedCh)`: goroutineでchannelを受け取りpublish。ctx.Doneで安全停止
- `publishItemStates` / `publishPlayerStates`: broker.Broadcast呼び出し（msgType byte使用）

### 10. `server/game/primitive.go` (~65行)
- terminal-shooterとほぼ同一。`cockroachdb/errors` → `fmt.Errorf`に変更
- `exhaustruct`タグ削除、GameID型削除

### 11. `server/game/collision.go` (~14行)
- terminal-shooterと同一

### 12. `server/game/item.go` (~20行)
- `ItemTypeBomb`, `ItemTypeBombFire` 削除（ItemTypeBulletのみ）
- `ToSharedItemType()`: Bulletケースのみ

### 13. `server/game/bullet.go` (~75行)
- terminal-shooterとほぼ同一。`exhaustruct`タグ削除のみ

### 14. `server/game/player.go` (~90行)
- `OnCollideWith`: `*BombFire`ケース削除 → `*Bullet`のみ
- `exhaustruct`タグ削除

### 15. `server/game/game.go` (~200行)
- `gameOperationProvider`: `addItem`メソッド削除（ボムの爆発時にしか使われないため）
  - `RemoveItem`と`UpdatePlayerStatus`の2メソッドのみ
- `StartUpdateLoop(ctx)`: context受け取り。ctx.Doneで安全停止、defer close(updatedCh)
- `PlaceBomb`メソッド削除
- `addItem`/`addItemWithoutLock`は内部メソッドとして残す（ShootBulletが使うため）
  - ただしgameOperationProviderからは除外
- stats参照削除

### 16. `client/main.go` (~280行) ★MQTT→TCPの大幅変更
- `mqtt.Client` → `net.Conn`
- MQTT接続設定(opts, Connect, Subscribe, Disconnect) → `net.Dial("tcp", "localhost:8080")`
- メッセージ受信: MQTTコールバック → goroutineでprotocol.ReadMessageループ → messageChan
- 送信: `mqtt.Publish(topic, ...)` → `protocol.WriteMessage(conn, Message{Type: ..., Payload: ...})`
- 受信ハンドラ: `message.Topic()` → `msg.Type` のswitch
- 描画: ボム/ボムファイア関連の描画(`@`, `#`, bombColor, fireColor)削除
- MessageStats全体削除
- `placeBomb`メソッド削除

### 17. `Makefile`
```makefile
.PHONY: gen
gen:
	cd shared/proto && protoc --go_out=../ --go_opt=paths=source_relative game.proto
```

---

## 実装順序

PoCでも段階的に動作確認しながら進める。

### Step 1: 基盤レイヤー
1. `go mod init` + 依存追加
2. `shared/proto/game.proto` → `make gen`でコード生成
3. `shared/protocol/protocol.go`
4. `shared/util.go`

**確認**: `go build ./...` が通ること

### Step 2: ゲームロジック
5. `server/game/primitive.go`
6. `server/game/collision.go`
7. `server/game/item.go`
8. `server/game/bullet.go`
9. `server/game/player.go`
10. `server/game/game.go`

**確認**: `go build ./...` が通ること

### Step 3: サーバーネットワーク層
11. `server/client.go`
12. `server/broker.go`
13. `server/game_service.go`
14. `server/server.go`
15. `server/main.go`

**確認**: `go run ./server/` でサーバーが起動すること

### Step 4: クライアント
16. `client/main.go`

**確認**: サーバー + 複数クライアントで動作確認

---

## 動作確認方法

1. ターミナル1: `go run ./server/`
2. ターミナル2: `go run ./client/`
3. ターミナル3: `go run ./client/`

確認項目:
- [ ] 矢印キーでプレイヤーが移動する
- [ ] 2つ目のクライアントが接続すると、互いのプレイヤーが見える
- [ ] スペースキーで弾が発射される（プレイヤーの前方に出現）
- [ ] 弾がサーバー側で0.5秒ごとに1マス移動する
- [ ] 弾が盤面外に出たら消える
- [ ] 弾がプレイヤーに当たったらプレイヤーがDEAD（`x`表示）になる
- [ ] DEADプレイヤーは移動・弾発射不可
- [ ] クライアント切断時に他クライアントからプレイヤーが消える

---

## 設計上の判断ポイント

| 判断 | 採用 | 理由 | 破綻するケース |
|------|------|------|----------------|
| エラーハンドリング | `fmt.Errorf` + `errors.Join` | CLAUDE.md「標準パッケージのみ」方針 | スタックトレースが必要になった場合 |
| UUID | 維持（`github.com/google/uuid`） | 一意なID生成の意図が読者に明確 | 依存を極限まで減らしたい場合 |
| gameOperationProviderから`addItem`除外 | 除外 | ボムなしなら不要。YAGNI原則 | ボム追加時に再追加が必要 |
| ctx伝搬 | あり（簡易版） | goroutineの安全な停止。+15行程度 | 完全なgraceful shutdown不要 |
| ポート番号 | `:8080` | MQTT(1883)ではないので変更 | 特になし |
| OnSubscribed廃止 | OnConnectedに統合 | Subscribe概念がない | 特になし |
| Hooker → Handler | 変更 | Go慣習に合致。「イベント処理のcontract」として明確 | — |
| Controller → GameService | 変更 | Handlerの実装だけでなくpublishも行う。Service層として自然 | — |
