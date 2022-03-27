package calculator

import (
	"math"
	"strconv"

	"github.com/netlify/gocommerce/claims"
	"github.com/sirupsen/logrus"
)

// DiscountItem provides details about a discount that was applied
type DiscountItem struct {
	Type       DiscountType `json:"type"`
	Percentage float64      `json:"percentage"`
	Fixed      float64      `json:"fixed"`
}

// Price represents the total price of all line items.
type Price struct {
	Items []ItemPrice

	Subtotal float64
	Discount float64
	NetTotal float64
	Taxes    float64
	Total    float64
}

// ItemPrice is the price of a single line item.
type ItemPrice struct {
	Quantity float64

	Subtotal float64
	Discount float64
	NetTotal float64
	Taxes    float64
	Total    float64

	DiscountItems []DiscountItem
}

// PaymentMethods settings
type PaymentMethods struct {
	Stripe struct {
		Enabled   bool   `json:"enabled"`
		PublicKey string `json:"public_key,omitempty"`
	} `json:"stripe"`
	PayPal struct {
		Enabled     bool   `json:"enabled"`
		ClientID    string `json:"client_id,omitempty"`
		Environment string `json:"environment,omitempty"`
	} `json:"paypal"`
}

// Settings represent the site-wide settings for price calculation.
type Settings struct {
	PricesIncludeTaxes bool              `json:"prices_include_taxes"`
	Taxes              []*Tax            `json:"taxes,omitempty"`
	MemberDiscounts    []*MemberDiscount `json:"member_discounts,omitempty"`
	PaymentMethods     *PaymentMethods   `json:"payment_methods,omitempty"`
}

// Tax represents a tax, potentially specific to countries and product types.
type Tax struct {
	Percentage   float64  `json:"percentage"`
	ProductTypes []string `json:"product_types"`
	Countries    []string `json:"countries"`
}

type taxAmount struct {
	price      float64
	percentage float64
}

// FixedMemberDiscount represents a fixed discount given to members.
type FixedMemberDiscount struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// MemberDiscount represents a discount given to members, either fixed
// or a percentage.
type MemberDiscount struct {
	Claims       map[string]string      `json:"claims"`
	Percentage   float64                `json:"percentage"`
	FixedAmount  []*FixedMemberDiscount `json:"fixed"`
	ProductTypes []string               `json:"product_types"`
	Products     []string               `json:"products"`
}

// PriceParameters represents the order information to calculate prices.
type PriceParameters struct {
	Country  string
	Currency string
	Coupon   Coupon
	Items    []Item
}

// ValidForType returns whether a member discount is valid for a product type.
func (d *MemberDiscount) ValidForType(productType string) bool {
	if d.ProductTypes == nil || len(d.ProductTypes) == 0 {
		return true
	}
	for _, validType := range d.ProductTypes {
		if validType == productType {
			return true
		}
	}
	return false
}

// ValidForProduct returns whether a member discount is valid for a product sku
func (d *MemberDiscount) ValidForProduct(productSku string) bool {
	if d.Products == nil || len(d.Products) == 0 {
		return true
	}
	for _, validSku := range d.Products {
		if validSku == productSku {
			return true
		}
	}
	return false
}

// Item is the interface for a single line item needed to do price calculation.
type Item interface {
	ProductSku() string
	PriceInLowestUnit() float64
	ProductType() string
	FixedVAT() float64
	TaxableItems() []Item
	GetQuantity() float64
}

// Coupon is the interface for a coupon needed to do price calculation.
type Coupon interface {
	ValidForType(string) bool
	ValidForPrice(string, float64) bool
	ValidForProduct(string) bool
	PercentageDiscount() float64
	FixedDiscount(string) float64
}

// FixedDiscount returns what the fixed discount amount is for a particular currency.
func (d *MemberDiscount) FixedDiscount(currency string) float64 {
	if d.FixedAmount != nil {
		for _, discount := range d.FixedAmount {
			if discount.Currency == currency {
				amount, _ := strconv.ParseFloat(discount.Amount, 64)
				return amount * 100
			}
		}
	}

	return 0.0
}

// AppliesTo determines if the tax applies to the country AND product type provided.
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

func calculateAmountsForSingleItem(settings *Settings, lineLogger logrus.FieldLogger, jwtClaims map[string]interface{}, params PriceParameters, item Item, multiplier uint64) ItemPrice {
	itemPrice := ItemPrice{Quantity: item.GetQuantity()}

	singlePrice := item.PriceInLowestUnit() * multiplier
	_, itemPrice.Subtotal = calculateTaxes(singlePrice, item, params, settings)

	// apply discount to original price
	coupon := params.Coupon
	if coupon != nil && coupon.ValidForType(item.ProductType()) && coupon.ValidForProduct(item.ProductSku()) {
		discountItem := DiscountItem{
			Type:       DiscountTypeCoupon,
			Percentage: coupon.PercentageDiscount(),
			Fixed:      coupon.FixedDiscount(params.Currency) * multiplier,
		}
		itemPrice.Discount = calculateDiscount(singlePrice, discountItem.Percentage, discountItem.Fixed)
		itemPrice.DiscountItems = append(itemPrice.DiscountItems, discountItem)
	}
	if settings != nil && settings.MemberDiscounts != nil {
		for _, discount := range settings.MemberDiscounts {

			if jwtClaims != nil && claims.HasClaims(jwtClaims, discount.Claims) && discount.ValidForType(item.ProductType()) && discount.ValidForProduct(item.ProductSku()) {
				lineLogger = lineLogger.WithField("discount", discount.Claims)
				discountItem := DiscountItem{
					Type:       DiscountTypeMember,
					Percentage: discount.Percentage,
					Fixed:      discount.FixedDiscount(params.Currency) * multiplier,
				}
				itemPrice.Discount += calculateDiscount(singlePrice, discountItem.Percentage, discountItem.Fixed)
				itemPrice.DiscountItems = append(itemPrice.DiscountItems, discountItem)
			}
		}
	}

	discountedPrice := 0.0
	if itemPrice.Discount < singlePrice {
		discountedPrice = singlePrice - itemPrice.Discount
	}

	itemPrice.Taxes, itemPrice.NetTotal = calculateTaxes(discountedPrice, item, params, settings)
	itemPrice.Total = itemPrice.NetTotal + itemPrice.Taxes

	return itemPrice
}

// CalculatePrice will calculate the final total price. It takes into account
// currency, country, coupons, and discounts.
func CalculatePrice(settings *Settings, jwtClaims map[string]interface{}, params PriceParameters, log logrus.FieldLogger) Price {
	price := Price{}

	priceLogger := log.WithField("action", "calculate_price")
	if am, ok := jwtClaims["app_metadata"]; ok {
		if a, ok := am.(map[string]interface{}); ok {
			if s, ok := a["subscription"]; ok {
				priceLogger = priceLogger.WithField("subscription_claim", s)
			}
		}
	}

	for _, item := range params.Items {
		lineLogger := priceLogger.WithFields(logrus.Fields{
			"product_type": item.ProductType(),
			"product_sku":  item.ProductSku(),
		})

		itemPrice := calculateAmountsForSingleItem(settings, lineLogger, jwtClaims, params, item, 1)

		lineLogger.WithFields(
			logrus.Fields{
				"item_price":    itemPrice.Total,
				"item_discount": itemPrice.Discount,
				"item_nettotal": itemPrice.NetTotal,
				"item_quantity": itemPrice.Quantity,
				"item_taxes":    itemPrice.Taxes,
			}).Info("calculated item price")

		price.Items = append(price.Items, itemPrice)

		// avoid issues with rounding when multiplying by quantity before taxation
		itemPriceMultiple := calculateAmountsForSingleItem(settings, lineLogger, jwtClaims, params, item, item.GetQuantity())
		price.Subtotal += itemPriceMultiple.Subtotal
		price.Discount += itemPriceMultiple.Discount
		price.NetTotal += itemPriceMultiple.NetTotal
		price.Taxes += itemPriceMultiple.Taxes
		price.Total += itemPriceMultiple.Total
	}

	price.Total = int64(price.NetTotal + price.Taxes)
	priceLogger.WithFields(
		logrus.Fields{
			"total_price":    price.Total,
			"total_discount": price.Discount,
			"total_net":      price.NetTotal,
			"total_taxes":    price.Taxes,
		}).Info("calculated total price")

	return price
}

func calculateDiscount(amountToDiscount, percentage, fixed float64) float64 {
	var discount float64
	if percentage > 0 {
		discount = amountToDiscount * percentage / 100
	}
	discount += fixed

	if discount > amountToDiscount {
		return amountToDiscount
	}
	return discount
}

func calculateTaxes(amountToTax float64, item Item, params PriceParameters, settings *Settings) (taxes float64, subtotal float64) {
	includeTaxes := settings != nil && settings.PricesIncludeTaxes
	originalPrice := item.PriceInLowestUnit()

	taxAmounts := []taxAmount{}
	if item.FixedVAT() != 0 {
		taxAmounts = append(taxAmounts, taxAmount{price: amountToTax, percentage: item.FixedVAT()})
	} else if settings != nil && item.TaxableItems() != nil && len(item.TaxableItems()) > 0 {
		for _, item := range item.TaxableItems() {
			// because a discount may have been applied we need to determine the real price of this sub-item
			priceShare := item.PriceInLowestUnit() / originalPrice
			itemPrice := amountToTax * priceShare
			amount := taxAmount{price: itemPrice}
			for _, t := range settings.Taxes {
				if t.AppliesTo(params.Country, item.ProductType()) {
					amount.percentage = t.Percentage
					break
				}
			}
			taxAmounts = append(taxAmounts, amount)
		}
	} else if settings != nil {
		for _, t := range settings.Taxes {
			if t.AppliesTo(params.Country, item.ProductType()) {
				taxAmounts = append(taxAmounts, taxAmount{price: amountToTax, percentage: t.Percentage})
				break
			}
		}
	}

	taxes = 0
	if len(taxAmounts) == 0 {
		subtotal = amountToTax
		return
	}

	subtotal = 0
	for _, tax := range taxAmounts {
		if includeTaxes {
			taxAmount := tax.price / (100+tax.percentage) * 100 * tax.percentage / 100
			tax.price -= taxAmount
			taxes += taxAmount
		} else {
			taxes += tax.price * tax.percentage / 100
		}
		subtotal += tax.price
	}

	return
}

// Nopes - no `round` method in go
// See https://github.com/golang/go/blob/master/src/math/floor.go#L58

const (
	uvone    = 0x3FF0000000000000
	mask     = 0x7FF
	shift    = 64 - 11 - 1
	bias     = 1023
	signMask = 1 << 63
	fracMask = 1<<shift - 1
)