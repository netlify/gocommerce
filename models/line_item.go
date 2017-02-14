package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/pborman/uuid"
)

type LineItem struct {
	ID      int64  `json:"id"`
	OrderID string `json:"-"`

	Title       string `json:"title"`
	Sku         string `json:"sku"`
	Type        string `json:"type"`
	Description string `json:"description"`

	Path string `json:"path"`

	Price    uint64 `json:"price"`
	Discount uint64 `json:"discount"`
	VAT      uint64 `json:"vat"`

	PriceItems []PriceItem `json:"price_items"`
	AddonItems []AddonItem `json:"addons"`
	AddonPrice uint64      `json:"addon_price"`

	Quantity uint64 `json:"quantity"`

	MetaData    map[string]interface{} `sql:"-" json:"meta"`
	RawMetaData string                 `json:"-"`

	CreatedAt time.Time  `json:"-"`
	DeletedAt *time.Time `json:"-"`
}

type PriceItem struct {
	ID int64 `json:"id"`

	Amount   uint64 `json:"amount"`
	Discount uint64 `json:"discount"`
	Type     string `json:"type"`
	VAT      uint64 `json:"vat"`
}

func (PriceItem) TableName() string {
	return tableName("price_items")
}

type AddonItem struct {
	ID int64 `json:"id"`

	Sku         string `json:"sku"`
	Title       string `json:"title"`
	Description string `json:"description"`

	Price uint64 `json:"price"`
}

func (AddonItem) TableName() string {
	return tableName("addon_items")
}

func (LineItem) TableName() string {
	return tableName("line_items")
}

func (l *LineItem) BeforeUpdate() (err error) {
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

type AddonMetaItem struct {
	Sku         string          `json:"sku"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Prices      []PriceMetadata `json:"prices"`
}

type LineItemMetadata struct {
	Sku         string          `json:"sku"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	VAT         uint64          `json:"vat"`
	Prices      []PriceMetadata `json:"prices"`
	Type        string          `json:"type"`

	Downloads []Download      `json:"downloads"`
	Addons    []AddonMetaItem `json:"addons"`

	Webhook string `json:"webhook"`
}

func (i *LineItem) Process(order *Order, meta *LineItemMetadata) error {
	i.Sku = meta.Sku
	i.Title = meta.Title
	i.Description = meta.Description
	i.VAT = meta.VAT
	i.Type = meta.Type

	for index, addon := range i.AddonItems {
		var metaAddon *AddonMetaItem
		for _, m := range meta.Addons {
			if addon.Sku == m.Sku {
				metaAddon = &m
				break
			}
		}
		if metaAddon == nil {
			return fmt.Errorf("Unkown addon %v for item %v", addon.Sku, i.Sku)
		}

		lowestPrice, err := determineLowestPrice(metaAddon.Prices, order.Currency)
		if err != nil {
			return err
		}

		i.AddonItems[index].Title = metaAddon.Title
		i.AddonItems[index].Description = metaAddon.Description
		i.AddonItems[index].Price = lowestPrice.cents
	}

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
		download.Sku = i.Sku
		order.Downloads = append(order.Downloads, download)
	}

	if err := i.calculatePrice(meta.Prices, order.Currency); err != nil {
		return err
	}

	i.calculateDiscount(order)

	return nil
}

func (i *LineItem) calculatePrice(prices []PriceMetadata, currency string) error {
	lowestPrice, err := determineLowestPrice(prices, currency)
	if err != nil {
		return err
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
	for _, addon := range i.AddonItems {
		i.AddonPrice += addon.Price
	}

	return nil
}

func (i *LineItem) calculateDiscount(order *Order) {
	if order.Coupon == nil {
		return
	}

	if !order.Coupon.ValidForType(i.Type) {
		return
	}

	if order.Coupon.Percentage != 0 {
		p := uint64(order.Coupon.Percentage)
		i.Discount = i.Price*p/100 + i.AddonPrice*p/100
	}
}

func determineLowestPrice(prices []PriceMetadata, currency string) (PriceMetadata, error) {
	lowestPrice := PriceMetadata{}
	for _, price := range prices {
		if price.Currency == currency {
			amount, err := strconv.ParseFloat(price.Amount, 64)
			if err != nil {
				return lowestPrice, err
			}
			price.cents = uint64(amount * 100)
			if lowestPrice.cents == 0 || price.cents < lowestPrice.cents {
				lowestPrice = price
			}
		}
	}
	return lowestPrice, nil
}
