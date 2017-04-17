package models

import "time"

type Coupon struct {
	Code string `json:"code"`

	StartDate *time.Time `json:"start_date,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`

	Percentage uint64 `json:"percentage,omitempty"`
	Amount uint64 `json:"amount,omitempty"`

	ProductTypes []string               `json:"product_types,omitempty"`
	Claims       map[string]interface{} `json:"claims,omitempty"`
}

func (c *Coupon) Valid() bool {
	if c.StartDate != nil && time.Now().Before(*c.StartDate) {
		return false
	}
	if c.EndDate != nil && time.Now().After(*c.EndDate) {
		return false
	}
	return true
}

func (c *Coupon) ValidForType(productType string) bool {
	if c == nil {
		return false
	}

	if c.ProductTypes == nil || len(c.ProductTypes) == 0 {
		return true
	}

	for _, t := range c.ProductTypes {
		if t == productType {
			return true
		}
	}

	return false
}

func (c *Coupon) ValidForPrice(currency string, price uint64) bool {
	// TODO: Support for coupons based on amount
	return true
}

func (c *Coupon) PercentageDiscount() uint64 {
	return c.Percentage
}
func (c *Coupon) FixedDiscount() uint64 {
	// TODO: Support for fixed amount discoutns
	return 0
}
