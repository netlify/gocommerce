package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

// User model
type User struct {
	InstanceID string `json:"-"`
	ID         string `json:"id"`
	Email      string `json:"email"`
	Name       string `json:"name"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-"`

	OrderCount  int64          `json:"order_count" gorm:"-"`
	LastOrderAt *HackyNullTime `json:"last_order_at" gorm:"-"`
}

// @todo: replace with mysql.NullTime once the tests no longer use SQLite
type HackyNullTime struct {
	Time  time.Time
	Valid bool
}

func (t *HackyNullTime) Scan(value interface{}) (err error) {
	if value == nil {
		t.Valid = false
		return nil
	}

	// try parsing with mysql time format
	var parsingTime mysql.NullTime
	if err := parsingTime.Scan(value); err == nil {
		t.Time = parsingTime.Time
		t.Valid = parsingTime.Valid
		return nil
	}

	// fallback to sqlite time format
	timeFormat := "2006-01-02 15:04:05.999999-07:00"

	switch v := value.(type) {
	case []byte:
		t.Time, err = time.Parse(timeFormat, string(v))
		t.Valid = (err == nil)
	case string:
		t.Time, err = time.Parse(timeFormat, v)
		t.Valid = (err == nil)
	}

	return nil
}

func (t *HackyNullTime) MarshalJSON() ([]byte, error) {
	if !t.Valid {
		return json.Marshal(nil)
	}

	return json.Marshal(t.Time)
}

func (t *HackyNullTime) UnmarshalJSON(data []byte) error {
	time := time.Time{}
	err := time.UnmarshalJSON(data)
	if err == nil && !time.IsZero() {
		t.Valid = true
		t.Time = time
		return nil
	}
	t.Valid = false
	return nil
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
