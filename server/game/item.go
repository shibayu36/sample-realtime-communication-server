package game

type ItemType string

const (
	ItemTypeBullet ItemType = "bullet"
)

// Item はゲームループで管理されるゲームオブジェクトを表す
type Item interface {
	collidable
	ID() ItemID
	Type() ItemType
	Position() Position
	// Update はゲームループの1tickごとに呼ばれる。状態が変更された場合はtrueを返す。
	Update() bool
}
