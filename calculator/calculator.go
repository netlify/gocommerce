package calculator

import "math"

type Price struct {
	Items []ItemPrice

	Subtotal uint64
	Discount uint64
	Taxes    uint64
	Total    uint64
}

type ItemPrice struct {
	Subtotal uint64
	Discount uint64
	Taxes    uint64
	Total    uint64
}

type Settings struct {
	PricesIncludeTaxes bool   `json:"prices_include_taxes"`
	Taxes              []*Tax `json:"taxes"`
}

type Tax struct {
	Percentage   uint64   `json:"percentage"`
	ProductTypes []string `json:"product_types"`
	Countries    []string `json:"countries"`
}

func (t *Tax) AppliesTo(country, productType string) bool {
	applies := true
	if t.ProductTypes != nil && len(t.ProductTypes) > 0 {
		applies = false
		for _, t := range t.ProductTypes {
			if t == productType {
				applies = true
				break
			}
		}
	}
	if !applies {
		return false
	}
	if t.Countries != nil && len(t.Countries) > 0 {
		applies = false
		for _, c := range t.Countries {
			if c == country {
				applies = true
				break
			}
		}
	}
	return applies
}

type Item interface {
	PriceIn(string) uint64
	ProductType() string
	VAT() uint64
}

type Coupon interface {
	ValidForType(string) bool
	ValidForPrice(string, uint64) bool
	PercentageDiscount() uint64
	FixedDiscount() uint64
}

func CalculatePrice(settings *Settings, country, currency string, coupon Coupon, items []Item) Price {
	price := Price{}
	includeTaxes := settings != nil && settings.PricesIncludeTaxes
	for _, item := range items {
		itemPrice := ItemPrice{}
		itemPrice.Subtotal = item.PriceIn(currency)
		tax := item.VAT()
		if tax == 0 && settings != nil {
			for _, t := range settings.Taxes {
				if t.AppliesTo(country, item.ProductType()) {
					tax = t.Percentage
					break
				}
			}
		}

		if tax != 0 {
			if includeTaxes {
				itemPrice.Subtotal = rint(float64(itemPrice.Subtotal) / (100 + float64(tax)) * 100)
			}
			itemPrice.Taxes = rint(float64(itemPrice.Subtotal) * float64(tax) / 100)
		}
		if coupon != nil && coupon.ValidForType(item.ProductType()) {
			amountToDiscount := itemPrice.Subtotal
			if includeTaxes {
				amountToDiscount += itemPrice.Taxes
			}
			itemPrice.Discount = rint(float64(amountToDiscount) * float64(coupon.PercentageDiscount()) / 100)
		}

		itemPrice.Total = itemPrice.Subtotal - itemPrice.Discount + itemPrice.Taxes

		price.Items = append(price.Items, itemPrice)

		price.Subtotal += itemPrice.Subtotal
		price.Discount += itemPrice.Discount
		price.Taxes += itemPrice.Taxes
		price.Total += itemPrice.Total
	}

	price.Total = price.Subtotal - price.Discount + price.Taxes

	return price
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
