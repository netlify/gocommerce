package models

import (
	"time"

	"github.com/pborman/uuid"
)

// Transaction is an transaction with a payment provider
type Transaction struct {
	ID      string `json:"id"`
	Order   Order  `json:"-"`
	OrderID string `json:"order_id"`

	ProcessorID string `json:"processor_id"`

	User   *User  `json:"-"`
	UserID string `json:"user_id"`

	Amount         uint64 `json:"amount"`
	AmountReversed uint64 `json:"amount_reversed"`
	Currency       string `json:"currency"`

	FailureCode        string `json:"failure_code"`
	FailureDescription string `json:"failure_description"`

	Status string `json:"status"`
	Type   string `json:"type"`

	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"-"`
}

// NewTransaction returns a new transaction for an order
func NewTransaction(order *Order) *Transaction {
	return &Transaction{
		ID:       uuid.NewRandom().String(),
		OrderID:  order.ID,
		UserID:   order.UserID,
		Currency: "USD",
		Amount:   order.Total,
	}
}
