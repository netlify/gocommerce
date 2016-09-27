package models

import "time"

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func (User) TableName() string {
	return tableName("users")
}
