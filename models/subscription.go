package models

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

// Interval represents a billing period (e.g. 1M)
type Interval string

// SubscriptionMetadata stores facts about a subscription plan from a static site
type SubscriptionMetadata struct {
	Sku         string `json:"sku"`
	Title       string `json:"title"`
	Description string `json:"description"`

	Prices   []PriceMetadata `json:"prices"`
	Interval Interval        `json:"interval"`

	Webhook string `json:"webhook"`
}

// SubscriptionPaymentState allows describing the state of a SubscriptionItem
type SubscriptionPaymentState string

// These describes which states a subscribtion item can be in
const (
	SubscriptionPending    SubscriptionPaymentState = "pending"
	SubscriptionSubscribed SubscriptionPaymentState = "subscribed"
	SubscriptionCancelled  SubscriptionPaymentState = "cancelled"
)

// SubscriptionItem describes a subscription a user ordered
type SubscriptionItem struct {
	InstanceID string `json:"-" sql:"index"`
	ID         string `json:"id"`

	UserID string `json:"user_id,omitempty"`

	Title       string `json:"title"`
	Sku         string `json:"sku"`
	Type        string `json:"type"`
	Description string `json:"description" sql:"type:text"`

	Path string `json:"path"`

	Interval Interval `json:"interval"`

	Currency string `json:"currency"`
	Price    uint64 `json:"price"`
	Total    uint64 `json:"total"`

	Quantity uint64 `json:"quantity"`

	Callback string `json:"callback"`

	MetaData    map[string]interface{} `sql:"-" json:"meta"`
	RawMetaData string                 `json:"-" sql:"type:text"`

	PaymentState SubscriptionPaymentState `json:"payment_state"`

	CreatedAt time.Time  `json:"-"`
	DeletedAt *time.Time `json:"-"`
}

// TableName returns the database table name for the SubscriptionItem model.
func (SubscriptionItem) TableName() string {
	return tableName("subscription_items")
}

// AfterFind database callback.
func (s *SubscriptionItem) AfterFind() error {
	if s.RawMetaData != "" {
		err := json.Unmarshal([]byte(s.RawMetaData), &s.MetaData)
		if err != nil {
			return err
		}
	}

	return nil
}

// BeforeSave database callback.
func (s *SubscriptionItem) BeforeSave() error {
	if s.MetaData != nil {
		data, err := json.Marshal(s.MetaData)
		if err != nil {
			return err
		}
		s.RawMetaData = string(data)
	}

	return nil
}

// Process prepares the subscription for charging
func (s *SubscriptionItem) Process(meta *SubscriptionMetadata) error {
	s.Title = meta.Title
	s.Description = meta.Description
	s.Interval = meta.Interval
	s.Callback = meta.Webhook

	return s.calculatePrice(meta.Prices)
}

func (s *SubscriptionItem) calculatePrice(prices []PriceMetadata) error {
	for _, price := range prices {
		if price.Currency == s.Currency {
			amount, err := strconv.ParseFloat(price.Amount, 64)
			if err != nil {
				cents := uint64(amount * 100)
				s.Price = cents
				return nil
			}
		}
	}

	return errors.New("No valid price found for item")
}
