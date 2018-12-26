package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/jinzhu/gorm"
	"github.com/netlify/gocommerce/calculator"
	"github.com/netlify/gocommerce/claims"
	"github.com/netlify/gocommerce/conf"
	"github.com/pborman/uuid"
)

// DiscountItem provides details about a discount that was applied
type DiscountItem struct {
	ID         int64 `json:"-"`
	LineItemID int64 `json:"-"`

	calculator.DiscountItem `gorm:"embedded"`
}

// TableName returns the database table name for the DiscountItem model.
func (DiscountItem) TableName() string {
	return tableName("discount_items")
}

// CalculationDetail holds details about pricing for line items
type CalculationDetail struct {
	Subtotal uint64 `json:"subtotal"`

	Discount      uint64         `json:"discount"`
	DiscountItems []DiscountItem `json:"discount_items" gorm:"foreignkey:LineItemID"`

	NetTotal uint64 `json:"net_total"`
	Taxes    uint64 `json:"taxes"`
	Total    int64  `json:"total"`
}

// LineItem is a single item in an Order.
type LineItem struct {
	ID      int64  `json:"id"`
	OrderID string `json:"-"`

	Title       string `json:"title"`
	Sku         string `json:"sku"`
	Type        string `json:"type"`
	Description string `json:"description" sql:"type:text"`

	Path string `json:"path"`

	Price uint64 `json:"price"`
	VAT   uint64 `json:"vat"`

	*CalculationDetail `json:"calculation" gorm:"embedded;embedded_prefix:calculation_"`

	PriceItems []*PriceItem `json:"price_items"`
	AddonItems []*AddonItem `json:"addons"`
	AddonPrice uint64       `json:"addon_price"`

	Quantity uint64 `json:"quantity"`

	MetaData    map[string]interface{} `sql:"-" json:"meta"`
	RawMetaData string                 `json:"-" sql:"type:text"`

	CreatedAt time.Time  `json:"-"`
	DeletedAt *time.Time `json:"-"`
}

// TableName returns the database table name for the LineItem model.
func (LineItem) TableName() string {
	return tableName("line_items")
}

// BeforeSave database callback.
func (i *LineItem) BeforeSave() error {
	if len(i.MetaData) == 0 {
		i.RawMetaData = ""
		return nil
	}

	data, err := json.Marshal(i.MetaData)
	if err == nil {
		i.RawMetaData = string(data)
	}
	return err
}

// AfterFind database callback.
func (i *LineItem) AfterFind() error {
	if i.RawMetaData != "" {
		return json.Unmarshal([]byte(i.RawMetaData), &i.MetaData)
	}
	return nil
}

func (i *LineItem) BeforeDelete(tx *gorm.DB) error {
	for _, p := range i.PriceItems {
		if r := tx.Delete(p); r.Error != nil {
			return r.Error
		}
	}
	for _, a := range i.AddonItems {
		if r := tx.Delete(a); r.Error != nil {
			return r.Error
		}
	}
	return nil
}

// PriceItem represent the subcomponent price items of a LineItem.
type PriceItem struct {
	ID int64 `json:"id"`

	Amount uint64 `json:"amount"`
	Type   string `json:"type"`
	VAT    uint64 `json:"vat"`
}

// TableName returns the database table name for the PriceItem model.
func (PriceItem) TableName() string {
	return tableName("price_items")
}

// ProductSku returns the Sku of the line item to match the calculator.Item interface
func (i *PriceItem) ProductSku() string {
	return "" // PriceItems currently can't have a SKU
}

// PriceInLowestUnit implements part of the calculator.Item interface.
func (i *PriceItem) PriceInLowestUnit() uint64 {
	return i.Amount
}

// ProductType implements part of the calculator.Item interface.
func (i *PriceItem) ProductType() string {
	return i.Type
}

// FixedVAT implements part of the calculator.Item interface.
func (i *PriceItem) FixedVAT() uint64 {
	return i.VAT
}

// TaxableItems implements part of the calculator.Item interface.
func (i *PriceItem) TaxableItems() []calculator.Item {
	return nil
}

// GetQuantity implements part of the calculator.Item interface.
func (i *PriceItem) GetQuantity() uint64 {
	return 1
}

// AddonItem are additional items for a LineItem.
type AddonItem struct {
	ID int64 `json:"id"`

	Sku         string `json:"sku"`
	Title       string `json:"title"`
	Description string `json:"description"`

	Price uint64 `json:"price"`
}

// TableName returns the database table name for the AddonItem model.
func (AddonItem) TableName() string {
	return tableName("addon_items")
}

// PriceMetadata model
type PriceMetadata struct {
	Amount   string            `json:"amount"`
	Currency string            `json:"currency"`
	VAT      string            `json:"vat"`
	Items    []PriceMetaItem   `json:"items"`
	Claims   map[string]string `json:"claims"`

	cents uint64
}

// PriceMetaItem model
type PriceMetaItem struct {
	Amount string `json:"amount"`
	Type   string `json:"type"`
	VAT    uint64 `json:"vat"`
}

// AddonMetaItem model
type AddonMetaItem struct {
	Sku         string          `json:"sku"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Prices      []PriceMetadata `json:"prices"`
}

// LineItemMetadata model
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

// ProductSku returns the Sku of the line item to match the calculator.Item interface
func (i *LineItem) ProductSku() string {
	return i.Sku
}

// PriceInLowestUnit implements part of the calculator.Item interface.
func (i *LineItem) PriceInLowestUnit() uint64 {
	return i.Price + i.AddonPrice
}

// ProductType implements part of the calculator.Item interface.
func (i *LineItem) ProductType() string {
	return i.Type
}

// FixedVAT implements part of the calculator.Item interface.
func (i *LineItem) FixedVAT() uint64 {
	return i.VAT
}

// TaxableItems implements part of the calculator.Item interface.
func (i *LineItem) TaxableItems() []calculator.Item {
	if i.PriceItems != nil {
		items := make([]calculator.Item, len(i.PriceItems))
		for i, priceItem := range i.PriceItems {
			items[i] = priceItem
		}
		return items
	}
	return nil
}

// GetQuantity implements part of the calculator.Item interface.
func (i *LineItem) GetQuantity() uint64 {
	return i.Quantity
}

// Process calculates the price of a LineItem.
func (i *LineItem) Process(config *conf.Configuration, userClaims map[string]interface{}, order *Order) error {
	meta, err := i.FetchMeta(config.SiteURL)
	if err != nil {
		return err
	}

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

		lowestPrice, err := determineLowestPrice(userClaims, metaAddon.Prices, order.Currency)
		if err != nil {
			return err
		}

		i.AddonItems[index].Title = metaAddon.Title
		i.AddonItems[index].Description = metaAddon.Description
		i.AddonItems[index].Price = lowestPrice.cents
	}

	order.Downloads = i.MissingDownloads(order, meta)

	return i.calculatePrice(userClaims, meta.Prices, order.Currency)
}

// FetchMeta determines the product metadata for the item based on its path
func (i *LineItem) FetchMeta(siteURL string) (*LineItemMetadata, error) {
	resp, err := http.Get(siteURL + i.Path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return nil, err
	}

	metaTag := doc.Find(".gocommerce-product")
	if metaTag.Length() == 0 {
		return nil, fmt.Errorf("No script tag with class gocommerce-product tag found for '%v'", i.Title)
	}
	metaProducts := []*LineItemMetadata{}
	var parsingErr error
	metaTag.EachWithBreak(func(_ int, tag *goquery.Selection) bool {
		meta := &LineItemMetadata{}
		parsingErr = json.Unmarshal([]byte(tag.Text()), meta)
		if parsingErr != nil {
			return false
		}
		metaProducts = append(metaProducts, meta)
		return true
	})
	if parsingErr != nil {
		return nil, fmt.Errorf("Error parsing product metadata: %v", parsingErr)
	}

	if len(metaProducts) == 1 && i.Sku == "" {
		i.Sku = metaProducts[0].Sku
	}

	for _, meta := range metaProducts {
		if meta.Sku == i.Sku {
			return meta, nil
		}
	}

	return nil, fmt.Errorf("No product Sku from path matched: %v", i.Sku)
}

// MissingDownloads returns all downloads that are not yet listed in the order
func (i *LineItem) MissingDownloads(order *Order, meta *LineItemMetadata) []Download {
	downloads := []Download{}
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
		download.OrderID = order.ID
		download.Title = i.Title
		download.Sku = i.Sku
		downloads = append(downloads, download)
	}
	return downloads
}

func (i *LineItem) calculatePrice(userClaims map[string]interface{}, prices []PriceMetadata, currency string) error {
	lowestPrice, err := determineLowestPrice(userClaims, prices, currency)
	if err != nil {
		return err
	}
	i.Price = lowestPrice.cents
	i.PriceItems = make([]*PriceItem, len(lowestPrice.Items))
	for index, item := range lowestPrice.Items {
		amount, err := strconv.ParseFloat(item.Amount, 64)
		if err != nil {
			return err
		}
		i.PriceItems[index] = &PriceItem{Amount: uint64(amount * 100), Type: item.Type, VAT: item.VAT}
	}
	for _, addon := range i.AddonItems {
		i.AddonPrice += addon.Price
	}

	return nil
}

func determineLowestPrice(userClaims map[string]interface{}, prices []PriceMetadata, currency string) (PriceMetadata, error) {
	lowestPrice := PriceMetadata{}
	found := false
	for _, price := range prices {
		if price.Currency == currency {
			amount, err := strconv.ParseFloat(price.Amount, 64)
			if err != nil {
				return lowestPrice, err
			}
			price.cents = uint64(amount * 100)
			if (!found || price.cents < lowestPrice.cents) && claims.HasClaims(userClaims, price.Claims) {
				lowestPrice = price
				found = true
			}
		}
	}
	if !found {
		return lowestPrice, errors.New("No valid price found for item")
	}
	return lowestPrice, nil
}
