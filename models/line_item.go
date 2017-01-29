package models

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/pborman/uuid"
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

	Price      uint64 `json:"price"`
	PriceItems []PriceItem
	Quantity   uint64 `json:"quantity"`

	MetaData    map[string]interface{} `sql:"-",json:"meta"`
	RawMetaData string                 `json:"-"`

	CreatedAt time.Time  `json:"-"`
	DeletedAt *time.Time `json:"-"`
}

type PriceItem struct {
	ID int64 `json:"id"`

	Amount uint64 `json:"amount"`
	Type   string `json:"type"`
	VAT    uint64 `json:"vat"`
}

func (LineItem) TableName() string {
	return tableName("line_items")
}

func (l *LineItem) BeforeUpdate() (err error) {
	//fmt.Printf("Persisting line item: %v\n", l.MetaData)
	data, err := json.Marshal(l.MetaData)
	if err == nil {
		l.RawMetaData = string(data)
	}
	return err
}

func (l *LineItem) AfterFind() (err error) {
	if l.RawMetaData != "" {
		return json.Unmarshal([]byte(l.RawMetaData), &l.MetaData)
	}
	return err
}

type PriceMetadata struct {
	Amount   string          `json:"amount"`
	Currency string          `json:"currency"`
	VAT      string          `json:"vat"`
	Items    []PriceMetaItem `json:"items"`

	cents uint64
}

type PriceMetaItem struct {
	Amount string `json:"amount"`
	Type   string `json:"type"`
	VAT    uint64 `json:"vat"`
}

type LineItemMetadata struct {
	Sku         string          `json:"sku"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	VAT         uint64          `json:"vat"`
	Prices      []PriceMetadata `json:"prices"`
	Type        string          `json:"type"`

	Downloads []Download `json:"downloads"`
}

func (i *LineItem) Process(order *Order, meta *LineItemMetadata) error {
	i.SKU = meta.Sku
	i.Title = meta.Title
	i.Description = meta.Description
	i.VAT = meta.VAT
	i.Type = meta.Type

	for _, download := range meta.Downloads {
		alreadyCreated := false
		for _, d := range order.Downloads {
			if d.URL == download.URL {
				alreadyCreated = true
				break
			}
		}
		if alreadyCreated {
			continue
		}
		download.ID = uuid.NewRandom().String()
		download.Title = i.Title
		download.SKU = i.SKU
		order.Downloads = append(order.Downloads, download)
	}

	return i.calculatePrice(meta.Prices, order.Currency)
}

func (i *LineItem) calculatePrice(prices []PriceMetadata, currency string) error {
	lowestPrice := PriceMetadata{}
	for _, price := range prices {
		if price.Currency == currency {
			amount, err := strconv.ParseFloat(price.Amount, 64)
			if err != nil {
				return err
			}
			price.cents = uint64(amount * 100)
			if lowestPrice.cents == 0 || price.cents < lowestPrice.cents {
				lowestPrice = price
			}
		}
	}
	i.Price = lowestPrice.cents
	i.PriceItems = make([]PriceItem, len(lowestPrice.Items))
	for index, item := range lowestPrice.Items {
		amount, err := strconv.ParseFloat(item.Amount, 64)
		if err != nil {
			return err
		}
		i.PriceItems[index] = PriceItem{Amount: uint64(amount * 100), Type: item.Type, VAT: item.VAT}
	}

	return nil
}
