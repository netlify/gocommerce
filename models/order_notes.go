package models

import "time"

type OrderNote struct {
	ID int64 `json:"-"`

	UserID string `json:"user_id"`

	Text string `json:"text"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-"`
}
