package models

import (
	"fmt"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
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

func (u *User) BeforeDelete(tx *gorm.DB) error {
	cascadeModels := map[string]interface{}{
		"order": &[]Order{},
	}
	for name, cm := range cascadeModels {
		if err := cascadeDelete(tx, "user_id = ?", u.ID, name, cm); err != nil {
			return err
		}
	}

	delModels := map[string]interface{}{
		"address":     Address{},
		"hook":        Hook{},
		"transaction": Transaction{},
		"order note":  OrderNote{},
	}
	for name, dm := range delModels {
		if result := tx.Delete(dm, "user_id = ?", u.ID); result.Error != nil {
			return errors.Wrap(result.Error, fmt.Sprintf("Error deleting %s records", name))
		}
	}
	return nil
}
