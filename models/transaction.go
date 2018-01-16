package models

import (
	"time"

	"github.com/jinzhu/gorm"
	"github.com/pborman/uuid"
)

// ChargeTransactionType is the charge transaction type.
const ChargeTransactionType = "charge"

// RefundTransactionType is the refund transaction type.
const RefundTransactionType = "refund"

// Transaction is an transaction with a payment provider
type Transaction struct {
	InstanceID    string `json:"-"`
	ID            string `json:"id"`
	Order         *Order `json:"-"`
	OrderID       string `json:"order_id"`
	InvoiceNumber int64  `json:"invoice_number"`

	ProcessorID string `json:"processor_id"`

	User   *User  `json:"-"`
	UserID string `json:"user_id,omitempty"`

	Amount   uint64 `json:"amount"`
	Currency string `json:"currency"`

	FailureCode        string `json:"failure_code,omitempty"`
	FailureDescription string `json:"failure_description,omitempty" sql:"type:text"`

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
		InstanceID: order.InstanceID,
		ID:         uuid.NewRandom().String(),
		Order:      order,
		OrderID:    order.ID,
		User:       order.User,
		UserID:     order.UserID,
		Currency:   order.Currency,
		Amount:     order.Total,
		Type:       ChargeTransactionType,
	}
}

func GetTransaction(db *gorm.DB, id string) (*Transaction, error) {
	trans := &Transaction{ID: id}
	if rsp := db.First(trans); rsp.Error != nil {
		if rsp.RecordNotFound() {
			return nil, nil
		}
		return nil, rsp.Error
	}
	return trans, nil
}
