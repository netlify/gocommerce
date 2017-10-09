package models

import (
	"math"
	"strconv"
	"time"
)

// FixedAmount represents an amount and currency pair
type FixedAmount struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// Coupon represents a discount redeemable with a code.
type Coupon struct {
	Code string `json:"code"`

	StartDate *time.Time `json:"start_date,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`

	Percentage  uint64         `json:"percentage,omitempty"`
	FixedAmount []*FixedAmount `json:"fixed,omitempty"`

	ProductTypes []string               `json:"product_types,omitempty"`
	Products     []string               `json:"products,omitempty"`
	Claims       map[string]interface{} `json:"claims,omitempty"`
}

// Valid returns whether a coupon is valid or not.
func (c *Coupon) Valid() bool {
	if c.StartDate != nil && time.Now().Before(*c.StartDate) {
		return false
	}
	if c.EndDate != nil && time.Now().After(*c.EndDate) {
		return false
	}
	return true
}

// ValidForProduct returns whether a coupon applies to a specific product.
func (c *Coupon) ValidForProduct(productSku string) bool {
	if c == nil {
		return false
	}

	if c.Products == nil || len(c.Products) == 0 {
		return true
	}

	for _, s := range c.Products {
		if s == productSku {
			return true
		}
	}

	return false
}

// ValidForType returns whether a coupon applies to a specific product type.
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

// ValidForPrice returns whether a coupon applies to a specific amount.
func (c *Coupon) ValidForPrice(currency string, price uint64) bool {
	// TODO: Support for coupons based on amount
	return true
}

// PercentageDiscount returns the percentage discount of a Coupon.
func (c *Coupon) PercentageDiscount() uint64 {
	return c.Percentage
}

// FixedDiscount returns the amount of fixed discount for a Coupon.
func (c *Coupon) FixedDiscount(currency string) uint64 {
	if c.FixedAmount != nil {
		for _, discount := range c.FixedAmount {
			if discount.Currency == currency {
				amount, _ := strconv.ParseFloat(discount.Amount, 64)
				return rint(amount * 100)
			}
		}
	}

	return 0
}

// Nopes - no `round` method in go
// See https://gist.github.com/siddontang/1806573b9a8574989ccb
func rint(x float64) uint64 {
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

	return uint64(v)
}
