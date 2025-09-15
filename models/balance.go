package models

type Balance struct {
	ID      uint  `gorm:"primaryKey"`
	Amount  int64 // your balance field
	Version int   `gorm:"version"` // enables optimistic locking
}
