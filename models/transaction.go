package models

import (
	"time"

	"github.com/pborman/uuid"
)

// ChargeTransactionType is the charge transaction type.
const ChargeTransactionType = "charge"

// RefundTransactionType is the refund transaction type.
const RefundTransactionType = "refund"

// Transaction is an transaction with a payment provider
type Transaction struct {
	ID      string `json:"id"`
	Order   *Order `json:"-"`
	OrderID string `json:"order_id"`

	ProcessorID string `json:"processor_id"`

	User   *User  `json:"-"`
	UserID string `json:"user_id"`

	Amount   uint64 `json:"amount"`
	Currency string `json:"currency"`

	FailureCode        string `json:"failure_code"`
	FailureDescription string `json:"failure_description"`

	Status string `json:"status"`
	Type   string `json:"type"`

	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"-"`
}

// TableName returns the database table name for the Transaction model.
func (Transaction) TableName() string {
	return tableName("transactions")
}

// NewTransaction returns a new transaction for an order
func NewTransaction(order *Order) *Transaction {
	return &Transaction{
		ID:       uuid.NewRandom().String(),
		Order:    order,
		OrderID:  order.ID,
		User:     order.User,
		UserID:   order.UserID,
		Currency: order.Currency,
		Amount:   order.Total,
		Type:     ChargeTransactionType,
	}
}
