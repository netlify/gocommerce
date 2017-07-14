package models

import "time"

// User model
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-"`

	OrderCount int64 `json:"order_count,ommitempty" gorm:"-"`
}

// TableName returns the database table name for the User model.
func (User) TableName() string {
	return tableName("users")
}
