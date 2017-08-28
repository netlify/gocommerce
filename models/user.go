package models

import (
	"time"

	"github.com/jinzhu/gorm"
)

// User model
type User struct {
	InstanceID string `json:"-"`
	ID         string `json:"id"`
	Email      string `json:"email"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-"`

	OrderCount int64 `json:"order_count,ommitempty" gorm:"-"`
}

// TableName returns the database table name for the User model.
func (User) TableName() string {
	return tableName("users")
}

func GetUser(db *gorm.DB, userID string) (*User, error) {
	user := &User{ID: userID}
	if result := db.Find(user); result.Error != nil {
		if result.RecordNotFound() {
			return nil, nil
		}
		return nil, result.Error
	}
	return user, nil
}
