package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/netlify/gocommerce/calculator"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// PendingState is the pending state of an Order
const PendingState = "pending"

// PaidState is the paid state of an Order
const PaidState = "paid"

// ShippedState is the shipped state of an Order
const ShippedState = "shipped"

// FailedState is the failed state of an Order
const FailedState = "failed"

// NumberType | StringType | BoolType are the different types supported in custom data for orders
const (
	NumberType = iota
	StringType
	BoolType
)

// Order model
type Order struct {
	InstanceID    string `json:"-"`
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

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-" sql:"index:idx_orders_deleted_at"`
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

	if price.Total > 0 {
		o.Total = uint64(price.Total)
	}
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
