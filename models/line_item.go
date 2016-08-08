package models

import (
	"strconv"
	"time"
)

type LineItem struct {
	ID      int64  `json:"id"`
	OrderID string `json:"-"`

	Title       string `json:"title"`
	SKU         string `json:"sku"`
	Type        string `json:"type"`
	Description string `json:"description"`
	VAT         uint64 `json:"vat"`

	Path string `json:"path"`

	Price    uint64 `json:"price"`
	Quantity uint64 `json:"quantity"`

	CreatedAt time.Time  `json:"-"`
	DeletedAt *time.Time `json:"-"`
}

type PriceMetadata struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
	VAT      string `json:"vat"`
}

type LineItemMetadata struct {
	Sku         string          `json:"sku"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	VAT         uint64          `json:"vat"`
	Prices      []PriceMetadata `json:"prices"`
	Type        string          `json:"type"`
}

func (i *LineItem) Process(order *Order, meta *LineItemMetadata) error {
	i.SKU = meta.Sku
	i.Title = meta.Title
	i.Description = meta.Description
	i.VAT = meta.VAT
	i.Type = meta.Type

	return i.calculatePrice(meta.Prices, order.Currency)
}

func (i *LineItem) calculatePrice(prices []PriceMetadata, currency string) error {
	for _, price := range prices {
		if price.Currency == currency {
			amount, err := strconv.ParseFloat(price.Amount, 64)
			if err != nil {
				return err
			}
			cents := uint64(amount * 100)
			if i.Price == 0 || cents < i.Price {
				i.Price = cents
			}
		}
	}
	return nil
}
