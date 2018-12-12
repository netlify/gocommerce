package calculator

import (
	"bytes"
	"encoding/json"
)

// DiscountType indicates what type of discount was given
type DiscountType int

// possible types for a discount item
const (
	DiscountTypeCoupon DiscountType = iota + 1
	DiscountTypeMember
)

func (t DiscountType) String() string {
	switch t {
	case DiscountTypeCoupon:
		return "coupon"
	case DiscountTypeMember:
		return "member"
	}
	return "unknown"
}

// MarshalJSON marshals the enum as a quoted json string
func (t DiscountType) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(t.String())
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (t *DiscountType) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}

	switch j {
	case "coupon":
		*t = DiscountTypeCoupon
	case "member":
		*t = DiscountTypeMember
	default:
		*t = 0
	}
	return nil
}
