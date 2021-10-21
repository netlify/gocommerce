package models

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/netlify/gocommerce/calculator"
	"github.com/netlify/gocommerce/conf"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// PendingState is the pending state of an Order
const PendingState = "pending"

// PaidState is the paid state of an Order
const PaidState = "paid"

// ShippingState is the shipping state of an order
const ShippingState = "shipping"

// ShippedState is the shipped state of an Order
const ShippedState = "shipped"

// FailedState is the failed state of an Order
const FailedState = "failed"

// PaymentState are the possible values for the PaymentState field
var PaymentStates = []string{
	PendingState,
	PaidState,
	FailedState,
}

// FulfillmentStates are the possible values for the FulfillmentState field
var FulfillmentStates = []string{
	PendingState,
	ShippingState,
	ShippedState,
}

// NumberType | StringType | BoolType are the different types supported in custom data for orders
const (
	NumberType = iota
	StringType
	BoolType
)

// Order model
type Order struct {
	InstanceID    string `json:"-" sql:"index"`
	ID            string `json:"id"`
	InvoiceNumber int64  `json:"invoice_number,omitempty"`

	IP string `json:"ip"`

	User      *User  `json:"user,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	SessionID string `json:"-"`

	Email string `json:"email"`

	LineItems []*LineItem `json:"line_items"`

	Downloads []Download `json:"downloads"`

	Currency string `json:"currency"`
	Taxes    uint64 `json:"taxes"`
	Shipping uint64 `json:"shipping"`
	SubTotal uint64 `json:"subtotal"`
	Discount uint64 `json:"discount"`
	NetTotal uint64 `json:"net_total"`

	Total uint64 `json:"total"`

	PaymentState     string `json:"payment_state"`
	FulfillmentState string `json:"fulfillment_state"`
	State            string `json:"state"`

	PaymentProcessor string `json:"payment_processor"`

	Transactions []*Transaction `json:"transactions"`
	Notes        []*OrderNote   `json:"notes"`

	ShippingAddress   Address `json:"shipping_address" gorm:"ForeignKey:ShippingAddressID"`
	ShippingAddressID string  `json:"shipping_address_id"`

	BillingAddress   Address `json:"billing_address" gorm:"ForeignKey:BillingAddressID"`
	BillingAddressID string  `json:"billing_address_id"`

	VATNumber string `json:"vatnumber"`

	MetaData    map[string]interface{} `sql:"-" json:"meta"`
	RawMetaData string                 `json:"-" sql:"type:text"`

	CouponCode string `json:"coupon_code,omitempty"`

	Coupon    *Coupon `json:"coupon,omitempty" sql:"-"`
	RawCoupon string  `json:"-" sql:"type:text"`

	CreatedAt time.Time  `json:"created_at" sql:"index"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-" sql:"index"`

	ModificationLock sync.Mutex `json:"-" sql:"-"`
}

// TableName returns the database table name for the Order model.
func (Order) TableName() string {
	return tableName("orders")
}

// AfterFind database callback.
func (o *Order) AfterFind() error {
	if o.RawMetaData != "" {
		err := json.Unmarshal([]byte(o.RawMetaData), &o.MetaData)
		if err != nil {
			return err
		}
	}
	if o.RawCoupon != "" {
		o.Coupon = &Coupon{}
		err := json.Unmarshal([]byte(o.RawCoupon), &o.Coupon)
		if err != nil {
			return err
		}
	}

	return nil
}

// BeforeSave database callback.
func (o *Order) BeforeSave() error {
	if o.MetaData != nil {
		data, err := json.Marshal(o.MetaData)
		if err != nil {
			return err
		}
		o.RawMetaData = string(data)
	}
	if o.Coupon != nil {
		data, err := json.Marshal(o.Coupon)
		if err != nil {
			return err
		}
		o.RawCoupon = string(data)
	}

	return nil
}

// NewOrder creates a new pending Order.
func NewOrder(instanceID, sessionID, email, currency string) *Order {
	order := &Order{
		InstanceID: instanceID,
		ID:         uuid.NewRandom().String(),
		SessionID:  sessionID,
		Email:      email,
		Currency:   currency,
	}
	order.PaymentState = PendingState
	order.FulfillmentState = PendingState
	order.State = PendingState
	return order
}

// CalculateTotal calculates the total price of an Order.
func (o *Order) CalculateTotal(settings *calculator.Settings, claims map[string]interface{}, log logrus.FieldLogger) {
	items := make([]calculator.Item, len(o.LineItems))
	for i, item := range o.LineItems {
		items[i] = item
	}

	params := calculator.PriceParameters{o.ShippingAddress.Country, o.Currency, o.Coupon, items}
	price := calculator.CalculatePrice(settings, claims, params, log)

	o.SubTotal = price.Subtotal
	o.Taxes = price.Taxes
	o.Discount = price.Discount
	o.NetTotal = price.NetTotal

	// apply price details to line items
	for i, item := range price.Items {
		o.LineItems[i].CalculationDetail = &CalculationDetail{
			Discount: item.Discount,
			Subtotal: item.Subtotal,
			NetTotal: item.NetTotal,
			Taxes:    item.Taxes,
			Total:    item.Total,
		}

		for _, discount := range item.DiscountItems {
			discount := DiscountItem{
				DiscountItem: discount,
			}
			o.LineItems[i].CalculationDetail.DiscountItems = append(o.LineItems[i].CalculationDetail.DiscountItems, discount)
		}
	}

	if price.Total > 0 {
		o.Total = uint64(price.Total)
	}
}

// UpdateDownloads will refetch downloads for all line items in the order and
// update the downloads in the order
func (o *Order) UpdateDownloads(config *conf.Configuration, log logrus.FieldLogger) error {
	updateMap := downloadRefreshItemSet{}
	for _, item := range o.LineItems {
		updateMap.Add(item, o)
	}
	updates, err := updateMap.Update(nil, config, log)
	log.Debugf("Updated downloads of %d orders", len(updates))
	return err
}

func (o *Order) BeforeDelete(tx *gorm.DB) error {
	cascadeModels := map[string]interface{}{
		"line item": &[]LineItem{},
	}
	for name, cm := range cascadeModels {
		if err := cascadeDelete(tx, "order_id = ?", o.ID, name, cm); err != nil {
			return err
		}
	}

	delModels := map[string]interface{}{
		"event":       Event{},
		"transaction": Transaction{},
		"download":    Download{},
	}
	for name, dm := range delModels {
		if result := tx.Delete(dm, "order_id = ?", o.ID); result.Error != nil {
			return errors.Wrap(result.Error, fmt.Sprintf("Error deleting %s records", name))
		}
	}
	return nil
}

type downloadRefreshItemSetEntry struct {
	item   *LineItem
	orders []*Order
}
type downloadRefreshInstanceItems map[string]*downloadRefreshItemSetEntry
type downloadRefreshItemSet map[string]downloadRefreshInstanceItems

// Add will take a line item and an order to persist in
// the list of orders to update
func (m downloadRefreshItemSet) Add(item *LineItem, order *Order) {
	instance, ok := m[order.InstanceID]
	if !ok {
		instance = make(map[string]*downloadRefreshItemSetEntry)
		m[order.InstanceID] = instance
	}

	mapping, ok := instance[item.Sku]
	if !ok {
		mapping = &downloadRefreshItemSetEntry{
			item:   item,
			orders: []*Order{},
		}
		instance[item.Sku] = mapping
	}

	mapping.orders = append(mapping.orders, order)
}

// UpdateDownloads fetches downloads for all line items and updates orders with new downloads
func (m downloadRefreshItemSet) Update(db *gorm.DB, config *conf.Configuration, log logrus.FieldLogger) (updates []*Order, err error) {
	// @todo: run in parallel with goroutines, lock orders with mutexes
	for instanceID, items := range m {
		if config == nil {
			if db == nil {
				err = errors.New("Instance config or database connection missing")
				return
			}
			instance := Instance{}
			if queryErr := db.First(&instance, Instance{ID: instanceID}).Error; queryErr != nil {
				err = errors.Wrap(queryErr, "Failed fetching instance for order")
				return
			}
			config = instance.BaseConfig
		}

		for _, entry := range items {
			if entry.item.Sku == "" {
				log.Warningf(
					"Tried updating a line item without SKU at %s. Skipped to avoid memory update in FetchMeta",
					entry.item.Path,
				)
				continue
			}
			log.Debugf("Updating downloads for item with sku '%s'", entry.item.Sku)
			meta, fetchErr := entry.item.FetchMeta(config.SiteURL)
			if fetchErr != nil {
				// item might not be offered anymore, preserve downloads
				log.WithError(fetchErr).
					WithFields(map[string]interface{}{
						"path": entry.item.Path,
						"sku":  entry.item.Sku,
					}).
					Warning("Fetching product metadata failed. Skipping item.")
				continue
			}
			for _, order := range entry.orders {
				downloads := entry.item.MissingDownloads(order, meta)
				if len(downloads) == 0 {
					continue
				}
				// @todo: Lock order mutex if run in goroutines
				order.Downloads = append(order.Downloads, downloads...)

				updates = append(updates, order)
			}
		}
	}

	return
}
