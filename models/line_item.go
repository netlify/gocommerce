package models

import "time"

type LineItem struct {
	ID      int64  `json:"id"`
	OrderID string `json:"-"`

	Title       string `json:"titel"`
	SKU         string `json:"sku"`
	Description string `json:"description"`

	Path string `json:"path"`

	Price    uint64 `json:"price"`
	Quantity uint64 `json:"quantity"`

	CreatedAt time.Time  `json:"-"`
	DeletedAt *time.Time `json:"-"`
}
