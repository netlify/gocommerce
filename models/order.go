package models

import (
	"encoding/json"
	"math"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/pborman/uuid"
)

const PendingState = "pending"
const PaidState = "paid"

// NUMBER|STRING|BOOL are the different types supported in custom data for orders
const (
	NUMBER = iota
	STRING
	BOOL
)

type Order struct {
	ID string `json:"id"`

	User      *User  `json:"user,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	SessionID string `json:"-"`

	Email string `json:"email"`

	LineItems []*LineItem `json:"line_items"`

	Currency string `json:"currency"`
	Taxes    uint64 `json:"taxes"`
	Shipping uint64 `json:"shipping"`
	SubTotal uint64 `json:"subtotal"`
	Total    uint64 `json:"total"`

	PaymentState     string `json:"payment_state"`
	FulfillmentState string `json:"fulfillment_state"`
	State            string `json:"state"`

	Transactions []*Transaction `json:"transactions"`
	Notes        []*OrderNote   `json:"notes"`

	ShippingAddress   Address `json:"shipping_address",gorm:"ForeignKey:ShippingAddressID"`
	ShippingAddressID string

	BillingAddress   Address `json:"billing_address",gorm:"ForeignKey:BillingAddressID"`
	BillingAddressID string

	VATNumber string `json:"vatnumber"`

	Data []OrderData `json:"-"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-",sql:"index"`
}

func (Order) TableName() string {
	return tableName("orders")
}

// OrderData is the custom data on an Order
type OrderData struct {
	OrderID string `gorm:"primary_key"`
	Key     string `gorm:"primary_key"`

	Type int

	NumericValue float64
	StringValue  string
	BoolValue    bool
}

func (OrderData) TableName() string {
	return tableName("orders_data")
}

// Value returns the value of the data field
func (d *OrderData) Value() interface{} {
	switch d.Type {
	case STRING:
		return d.StringValue
	case NUMBER:
		return d.NumericValue
	case BOOL:
		return d.BoolValue
	}
	return nil
}

// InvalidDataType is an error returned when trying to set an invalid datatype for
// a user data key
type InvalidDataType struct {
	Key string
}

func (i *InvalidDataType) Error() string {
	return "Invalid datatype for data field " + i.Key + " only strings, numbers and bools allowed"
}

func orderDataToMap(data []OrderData) map[string]interface{} {
	result := map[string]interface{}{}
	for _, field := range data {
		switch field.Type {
		case NUMBER:
			result[field.Key] = field.NumericValue
		case STRING:
			result[field.Key] = field.StringValue
		case BOOL:
			result[field.Key] = field.BoolValue
		}
	}
	return result
}

// MarshalJSON is a custom JSON marshaller for Users
func (o *Order) MarshalJSON() ([]byte, error) {
	type Alias Order
	return json.Marshal(&struct {
		*Alias
		Data map[string]interface{} `json:"data"`
	}{
		Alias: (*Alias)(o),
		Data:  orderDataToMap(o.Data),
	})
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

// UpdateOrderData updates all user data from a map of updates
func (o *Order) UpdateOrderData(tx *gorm.DB, updates *map[string]interface{}) error {
	for key, value := range *updates {
		data := &OrderData{}
		result := tx.First(data, "order_id = ? and key = ?", o.ID, key)
		data.OrderID = o.ID
		data.Key = key
		if result.Error != nil && !result.RecordNotFound() {
			tx.Rollback()
			return result.Error
		}
		if value == nil {
			tx.Delete(data)
			continue
		}
		switch v := value.(type) {
		case string:
			data.StringValue = v
			data.Type = STRING
		case float64:
			data.NumericValue = v
			data.Type = NUMBER
		case bool:
			data.BoolValue = v
			data.Type = BOOL
		default:
			tx.Rollback()
			return &InvalidDataType{key}
		}
		if result.RecordNotFound() {
			tx.Create(data)
		} else {
			tx.Save(data)
		}
	}
	return nil
}

func (o *Order) CalculateTotal(settings *SiteSettings) {
	// Calculate taxes/shipping here
	var taxes uint64
	if o.VATNumber == "" {
		for _, item := range o.LineItems {
			taxes += taxFor(item, settings.Taxes, o.BillingAddress.Country)
		}
	}

	o.Total = o.SubTotal + taxes
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
	if item.VAT != 0 {
		return item.Price * item.Quantity * (item.VAT / 100)
	}
	if len(taxes) > 0 && country != "" {
		for _, tax := range taxes {
			if inList(tax.ProductTypes, item.Type) && inList(tax.Countries, country) {
				result := float64(item.Price) * float64(item.Quantity) * (float64(tax.Percentage) / 100)
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
