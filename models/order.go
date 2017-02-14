package models

import (
	"encoding/json"
	"math"
	"time"

	"github.com/pborman/uuid"
)

const PendingState = "pending"
const PaidState = "paid"
const ShippedState = "shipped"
const FailedState = "failed"

// NumberType | StringType | BoolType are the different types supported in custom data for orders
const (
	NumberType = iota
	StringType
	BoolType
)

type Order struct {
	ID string `json:"id"`

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

	ShippingAddress   Address `json:"shipping_address",gorm:"ForeignKey:ShippingAddressID"`
	ShippingAddressID string  `json:"shipping_address_id"`

	BillingAddress   Address `json:"billing_address",gorm:"ForeignKey:BillingAddressID"`
	BillingAddressID string  `json:"billing_address_id"`

	VATNumber string `json:"vatnumber"`

	MetaData    map[string]interface{} `sql:"-" json:"meta"`
	RawMetaData string                 `json:"-"`

	CouponCode string `json:"coupon_code,omitempty"`

	Coupon    *Coupon `json:"coupon,omitempty" sql:"-"`
	RawCoupon string  `json:"-"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-",sql:"index"`
}

func (Order) TableName() string {
	return tableName("orders")
}

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

func (o *Order) BeforeUpdate() error {
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

func NewOrder(sessionID, email, currency string) *Order {
	order := &Order{
		ID:        uuid.NewRandom().String(),
		SessionID: sessionID,
		Email:     email,
		Currency:  currency,
	}
	order.PaymentState = PendingState
	order.FulfillmentState = PendingState
	order.State = PendingState
	return order
}

func (o *Order) CalculateTotal(settings *SiteSettings) {
	// Calculate taxes/shipping here
	var taxes uint64
	if o.VATNumber == "" {
		for _, item := range o.LineItems {
			taxes += taxFor(item, settings.Taxes, o.ShippingAddress.Country)
			o.Discount += item.Discount
		}
	}

	o.Taxes = taxes
	o.Total = o.SubTotal - o.Discount + taxes
}

func inList(list []string, candidate string) bool {
	for _, item := range list {
		if item == candidate {
			return true
		}
	}
	return false
}

func taxFor(item *LineItem, taxes []*Tax, country string) uint64 {
	// Note - we're not handling products with PricItems and Addons right now
	if len(item.PriceItems) > 0 {
		var tax uint64
		for _, i := range item.PriceItems {
			tax += taxFor(&LineItem{
				Price:    i.Amount,
				Discount: i.Discount,
				Type:     i.Type,
				VAT:      i.VAT,
				Quantity: item.Quantity,
			}, taxes, country)
		}
		return tax
	}
	if item.VAT != 0 {
		return (item.Price + item.AddonPrice - item.Discount) * item.Quantity * (item.VAT / 100)
	}
	if len(taxes) > 0 && country != "" {
		for _, tax := range taxes {
			if inList(tax.ProductTypes, item.Type) && inList(tax.Countries, country) {
				result := float64(item.Price+item.AddonPrice-item.Discount) * float64(item.Quantity) * (float64(tax.Percentage) / 100)
				return uint64(rint(result))
			}
		}
	}
	return 0
}

// Nopes - no `round` method in go
// See https://gist.github.com/siddontang/1806573b9a8574989ccb
func rint(x float64) float64 {
	v, frac := math.Modf(x)
	if x > 0.0 {
		if frac > 0.5 || (frac == 0.5 && uint64(v)%2 != 0) {
			v += 1.0
		}
	} else {
		if frac < -0.5 || (frac == -0.5 && uint64(v)%2 != 0) {
			v -= 1.0
		}
	}

	return v
}
