package models

import "time"

type Transaction struct {
	ID      string `json:"id"`
	Order   Order
	OrderID string `json:"order_id"`

	ProcessorID string `json:"processor_id"`

	User   *User
	UserID string `json:"user_id"`

	Amount         uint64 `json:"amount"`
	AmountReversed uint64 `json:"amount_reversed"`
	Currency       string `json:"currency"`

	FailureCode        string `json:"failure_code"`
	FailureDescription string `json:"failure_description"`

	Status string `json:"status"`
	Type   string `json:"type"`

	CreatedAt time.Time
	DeletedAt *time.Time
}
