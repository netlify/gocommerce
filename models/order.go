package models

import (
	"time"

	"github.com/pborman/uuid"
)

const PendingState = "pending"
const PaidState = "paid"

type Order struct {
	ID string `json:"id"`

	User      User   `json:"user,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	SessionID string `json:"-"`

	Email string `json:"email"`

	LineItems []LineItem `json:"line_items"`

	Taxes    uint64 `json:"taxes"`
	Shipping uint64 `json:"shipping"`
	SubTotal uint64 `json:"subtotal"`
	Total    uint64 `json:"total"`

	PaymentState     string `json:"payment_state"`
	FulfillmentState string `json:"fulfillment_state"`
	State            string `json:"state"`

	Transactions []Transaction `json:"transactions"`
	Notes        []OrderNote   `json:"notes"`

	ShippingAddress   Address `json:"shipping_address",gorm:"ForeignKey:ShippingAddressID"`
	ShippingAddressID string

	BillingAddress   Address `json:"billing_address",gorm:"ForeignKey:BillingAddressID"`
	BillingAddressID string

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-",sql:"index"`
}

func NewOrder(sessionID, email string) *Order {
	order := &Order{
		ID:        uuid.NewRandom().String(),
		SessionID: sessionID,
		Email:     email,
	}
	order.PaymentState = PendingState
	order.FulfillmentState = PendingState
	order.State = PendingState
	return order
}
