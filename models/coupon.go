package models

import "time"

type Coupon struct {
	Code string `json:"code"`

	StartDate *time.Time `json:"start_date,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`

	Percentage int `json:"percentage,omitempty"`

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
