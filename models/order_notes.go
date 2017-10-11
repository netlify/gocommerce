package models

import "time"

// OrderNote model which represent notes on a model.
type OrderNote struct {
	ID int64 `json:"-"`

	UserID string `json:"user_id"`

	Text string `json:"text" sql:"type:text"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-"`
}

// TableName returns the database table name for the OrderNote model.
func (OrderNote) TableName() string {
	return tableName("orders_notes")
}
