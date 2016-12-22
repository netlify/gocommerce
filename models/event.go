package models

import (
	"strings"
	"time"

	"github.com/jinzhu/gorm"
)

type Event struct {
	ID uint64

	User   *User  `json:"user,omitempty"`
	UserID string `json:"user_id,omitempty"`

	Order   *Order `json:"order,omitempty"`
	OrderID string `json:"order_id,omitempty"`

	Type    string `json:"type"`
	Changes string `json:"data"`

	CreatedAt time.Time `json:"created_at"`
}

func (Event) TableName() string {
	return tableName("events")
}

type EventType string

const (
	EventCreated EventType = "created"
	EventUpdated EventType = "updated"
	EventDeleted EventType = "deleted"
)

// LogEvent logs a new event
func LogEvent(db *gorm.DB, userID, orderID string, eventType EventType, changes []string) {
	event := &Event{
		UserID:  userID,
		OrderID: orderID,
		Type:    string(eventType),
	}
	if changes != nil {
		event.Changes = strings.Join(changes, ",")
	}
	db.Create(event)
}
