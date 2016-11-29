package models

import (
	"fmt"
	"strings"
	"time"
)

type DiscountExternal struct {
	ID       string `json:"id"` // the actual coupon code
	Amount   int    `json:"amount"`
	Type     string `json:"type"` // 'percent' || 'flat'
	Currency string `json:"currency"`

	Start *time.Time `json:"start_time"`
	End   *time.Time `json:"end_time"`

	// the number of times this can be used per order
	// 0 indicates infinite
	OrderLimit int `json:"order_limit"`

	// the amount of times this can be applied
	// 0 indicates infinite
	Availability int `json:"availablity"`

	ProductGroups []string `json:"product_groups"`
	UserIDs       []string `json:"user_ids"`
	Groups        []string `json:"groups"`
}

type Discount struct {
	ID       string
	Amount   int    `gorm:"not null"`
	Type     string `gorm:"not null"`
	Currency string

	Start *time.Time
	End   *time.Time

	// the number of times this can be used per order
	// 0 indicates infinite
	OrderLimit int

	// the amount of times this can be applied
	// 0 indicates infinite
	Availability int

	ProductGroups string
	UserIDs       string
	Groups        string

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func (d Discount) Externalize() DiscountExternal {
	ext := DiscountExternal{
		ID:           d.ID,
		Amount:       d.Amount,
		Type:         d.Type,
		Start:        d.Start,
		End:          d.End,
		OrderLimit:   d.OrderLimit,
		Availability: d.Availability,
	}
	if d.ProductGroups != "" {
		ext.ProductGroups = strings.Split(d.ProductGroups, ", ")
	}

	if d.UserIDs != "" {
		ext.ProductGroups = strings.Split(d.UserIDs, ", ")
	}

	if d.Groups != "" {
		ext.Groups = strings.Split(d.Groups, ", ")
	}

	return ext
}

func (ext DiscountExternal) Internalize() *Discount {
	d := Discount{
		ID:           ext.ID,
		Amount:       ext.Amount,
		Type:         ext.Type,
		Start:        ext.Start,
		End:          ext.End,
		OrderLimit:   ext.OrderLimit,
		Availability: ext.Availability,
	}
	if len(ext.ProductGroups) > 0 {
		d.ProductGroups = strings.Join(ext.ProductGroups, ",")
	}
	if len(ext.UserIDs) > 0 {
		d.UserIDs = strings.Join(ext.UserIDs, ",")
	}
	if len(ext.Groups) > 0 {
		d.Groups = strings.Join(ext.Groups, ",")
	}

	return &d
}

func (d DiscountExternal) Validate() error {
	errors := []string{}
	switch strings.ToLower(d.Type) {
	case "flat":
		if d.Amount < 0 {
			errors = append(errors, "the amount must be greater than 0")
		}
		if d.Currency == "" {
			errors = append(errors, "must specify the currency when using a flat discount")
		}
	case "percent":
		if d.Amount < 0 || d.Amount > 100 {
			errors = append(errors, "the amount must be between 0 and 100")
		}
	default:
		errors = append(errors, fmt.Sprintf("the type '%s' isn't supported, only 'flat' and 'percent'", d.Type))
	}

	if d.OrderLimit < 0 {
		errors = append(errors, fmt.Sprintf("the order limit must >= 0"))
	}

	if len(d.ProductGroups) == 0 && len(d.Groups) == 0 && len(d.UserIDs) == 0 {
		errors = append(errors, "Must provide at least 1 group, user or product")
	}

	if d.ID == "" {
		errors = append(errors, "must provide an id for this coupon code")
	}

	if len(errors) > 0 {
		return fmt.Errorf("Errors in validation: [%s]", strings.Join(errors, ","))
	}

	return nil
}
