package calculator

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testLogger = logrus.NewEntry(logrus.StandardLogger())

type TestItem struct {
	sku      string
	price    uint64
	itemType string
	vat      uint64
	items    []Item
	quantity uint64
}

func (t *TestItem) ProductSku() string {
	return t.sku
}

func (t *TestItem) PriceInLowestUnit() uint64 {
	return t.price
}

func (t *TestItem) ProductType() string {
	return t.itemType
}

func (t *TestItem) FixedVAT() uint64 {
	return t.vat
}

func (t *TestItem) TaxableItems() []Item {
	return t.items
}

func (t *TestItem) GetQuantity() uint64 {
	if t.quantity > 0 {
		return t.quantity
	}
	return 1
}

type TestCoupon struct {
	itemSku    string
	itemType   string
	moreThan   uint64
	percentage uint64
	fixed      uint64
}

func (c *TestCoupon) ValidForType(productType string) bool {
	return c.itemType == productType
}

func (c *TestCoupon) ValidForProduct(productSku string) bool {
	return c.itemSku == productSku
}

func (c *TestCoupon) ValidForPrice(currency string, price uint64) bool {
	return c.moreThan == 0 || price > c.moreThan
}

func (c *TestCoupon) PercentageDiscount() uint64 {
	return c.percentage
}

func (c *TestCoupon) FixedDiscount(currency string) uint64 {
	return c.fixed
}

func TestNoItems(t *testing.T) {
	params := PriceParameters{"USA", "USD", nil, nil}
	price := CalculatePrice(nil, nil, params, testLogger)
	assert.Equal(t, int64(0), price.Total)
}

func TestNoTaxes(t *testing.T) {
	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100, itemType: "test"}}}
	price := CalculatePrice(nil, nil, params, testLogger)

	assert.Equal(t, uint64(100), price.Subtotal)
	assert.Equal(t, uint64(0), price.Taxes)
	assert.Equal(t, uint64(0), price.Discount)
	assert.Equal(t, int64(100), price.Total)
}

func TestFixedVAT(t *testing.T) {
	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100, itemType: "test", vat: 9}}}
	price := CalculatePrice(nil, nil, params, testLogger)

	assert.Equal(t, uint64(100), price.Subtotal)
	assert.Equal(t, uint64(9), price.Taxes)
	assert.Equal(t, uint64(0), price.Discount)
	assert.Equal(t, int64(109), price.Total)
}

func TestFixedVATWhenPricesIncludeTaxes(t *testing.T) {
	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100, itemType: "test", vat: 9}}}
	price := CalculatePrice(&Settings{PricesIncludeTaxes: true}, nil, params, testLogger)

	assert.Equal(t, uint64(92), price.Subtotal)
	assert.Equal(t, uint64(8), price.Taxes)
	assert.Equal(t, uint64(0), price.Discount)
	assert.Equal(t, int64(100), price.Total)
}

func TestCountryBasedVAT(t *testing.T) {
	settings := &Settings{
		Taxes: []*Tax{&Tax{
			Percentage:   21,
			ProductTypes: []string{"test"},
			Countries:    []string{"USA"},
		}},
	}

	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100, itemType: "test"}}}
	price := CalculatePrice(settings, nil, params, testLogger)

	assert.Equal(t, uint64(100), price.Subtotal)
	assert.Equal(t, uint64(21), price.Taxes)
	assert.Equal(t, uint64(0), price.Discount)
	assert.Equal(t, int64(121), price.Total)
}

func TestCouponWithNoTaxes(t *testing.T) {
	coupon := &TestCoupon{itemType: "test", percentage: 10}
	params := PriceParameters{"USA", "USD", coupon, []Item{&TestItem{price: 100, itemType: "test"}}}
	price := CalculatePrice(nil, nil, params, testLogger)

	assert.Equal(t, uint64(100), price.Subtotal)
	assert.Equal(t, uint64(0), price.Taxes)
	assert.Equal(t, uint64(10), price.Discount)
	assert.Equal(t, int64(90), price.Total)
}

func TestCouponWithVAT(t *testing.T) {
	coupon := &TestCoupon{itemType: "test", percentage: 10}
	params := PriceParameters{"USA", "USD", coupon, []Item{&TestItem{price: 100, itemType: "test", vat: 9}}}
	price := CalculatePrice(nil, nil, params, testLogger)

	assert.Equal(t, uint64(100), price.Subtotal)
	assert.Equal(t, uint64(9), price.Taxes)
	assert.Equal(t, uint64(10), price.Discount)
	assert.Equal(t, int64(99), price.Total)
}

func TestCouponWithVATWhenPRiceIncludeTaxes(t *testing.T) {
	coupon := &TestCoupon{itemType: "test", percentage: 10}
	settings := &Settings{PricesIncludeTaxes: true}
	params := PriceParameters{"USA", "USD", coupon, []Item{&TestItem{price: 100, itemType: "test", vat: 9}}}
	price := CalculatePrice(settings, nil, params, testLogger)

	assert.Equal(t, uint64(92), price.Subtotal)
	assert.Equal(t, uint64(8), price.Taxes)
	assert.Equal(t, uint64(10), price.Discount)
	assert.Equal(t, int64(90), price.Total)
}

func TestCouponWithVATWhenPRiceIncludeTaxesWithQuantity(t *testing.T) {
	coupon := &TestCoupon{itemType: "test", percentage: 10}
	settings := &Settings{PricesIncludeTaxes: true}
	params := PriceParameters{"USA", "USD", coupon, []Item{&TestItem{quantity: 2, price: 100, itemType: "test", vat: 9}}}
	price := CalculatePrice(settings, nil, params, testLogger)

	assert.Equal(t, uint64(184), price.Subtotal)
	assert.Equal(t, uint64(16), price.Taxes)
	assert.Equal(t, uint64(20), price.Discount)
	assert.Equal(t, int64(180), price.Total)
}

func TestPricingItems(t *testing.T) {
	settings := &Settings{Taxes: []*Tax{&Tax{
		Percentage:   7,
		ProductTypes: []string{"book"},
		Countries:    []string{"DE"},
	}, &Tax{
		Percentage:   21,
		ProductTypes: []string{"ebook"},
		Countries:    []string{"DE"},
	}}}
	item := &TestItem{
		price:    100,
		itemType: "book",
		items: []Item{&TestItem{
			price:    80,
			itemType: "book",
		}, &TestItem{
			price:    20,
			itemType: "ebook",
		}},
	}
	params := PriceParameters{"DE", "USD", nil, []Item{item}}
	price := CalculatePrice(settings, nil, params, testLogger)

	assert.Equal(t, uint64(100), price.Subtotal)
	assert.Equal(t, uint64(10), price.Taxes)
	assert.Equal(t, uint64(0), price.Discount)
	assert.Equal(t, int64(110), price.Total)
}

func TestMemberDiscounts(t *testing.T) {
	settings := &Settings{PricesIncludeTaxes: true, MemberDiscounts: []*MemberDiscount{&MemberDiscount{
		Claims:     map[string]string{"app_metadata.plan": "member"},
		Percentage: 10,
	}}}
	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100, itemType: "test", vat: 9}}}
	price := CalculatePrice(settings, nil, params, testLogger)

	assert.Equal(t, uint64(92), price.Subtotal)
	assert.Equal(t, uint64(8), price.Taxes)
	assert.Equal(t, uint64(0), price.Discount)
	assert.Equal(t, int64(100), price.Total)

	claims := map[string]interface{}{}
	require.NoError(t, json.Unmarshal([]byte(`{"app_metadata": {"plan": "member"}}`), &claims))

	params = PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100, itemType: "test", vat: 9}}}
	price = CalculatePrice(settings, claims, params, testLogger)

	assert.Equal(t, uint64(92), price.Subtotal)
	assert.Equal(t, uint64(8), price.Taxes)
	assert.Equal(t, uint64(10), price.Discount)
	assert.Equal(t, int64(90), price.Total)
}

func TestFixedMemberDiscounts(t *testing.T) {
	settings := &Settings{PricesIncludeTaxes: true, MemberDiscounts: []*MemberDiscount{&MemberDiscount{
		Claims: map[string]string{"app_metadata.plan": "member"},
		FixedAmount: []*FixedMemberDiscount{{
			Amount:   "0.10",
			Currency: "USD",
		}},
	}}}

	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100, itemType: "test", vat: 9}}}
	price := CalculatePrice(settings, nil, params, testLogger)

	assert.Equal(t, uint64(92), price.Subtotal)
	assert.Equal(t, uint64(8), price.Taxes)
	assert.Equal(t, uint64(0), price.Discount)
	assert.Equal(t, int64(100), price.Total)

	claims := map[string]interface{}{}
	require.NoError(t, json.Unmarshal([]byte(`{"app_metadata": {"plan": "member"}}`), &claims))

	params = PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100, itemType: "test", vat: 9}}}
	price = CalculatePrice(settings, claims, params, testLogger)

	assert.Equal(t, uint64(92), price.Subtotal)
	assert.Equal(t, uint64(8), price.Taxes)
	assert.Equal(t, uint64(10), price.Discount)
	assert.Equal(t, int64(90), price.Total)
}

func TestMixedDiscounts(t *testing.T) {
	b, err := ioutil.ReadFile("test/settings_fixture.json")
	assert.NoError(t, err)

	var settings Settings
	err = json.Unmarshal(b, &settings)
	assert.NoError(t, err)

	item := &TestItem{
		sku:      "design-systems-ebook",
		itemType: "Book",
		quantity: 1,
		price:    3490,
	}

	params := PriceParameters{"USA", "USD", nil, []Item{item}}
	price := CalculatePrice(&settings, nil, params, testLogger)
	assert.Equal(t, 3490, int(price.Total))

	claims := map[string]interface{}{
		"app_metadata": map[string]interface{}{
			"subscription": map[string]interface{}{
				"plan": "member",
			},
		},
	}
	price = CalculatePrice(&settings, claims, params, testLogger)
	assert.Equal(t, int64(0), price.Total)
}

func TestRealWorldTaxCalculations(t *testing.T) {
	settings := &Settings{
		PricesIncludeTaxes: true,
		Taxes: []*Tax{&Tax{
			Percentage:   7,
			ProductTypes: []string{"Book"},
			Countries:    []string{"USA"},
		}, &Tax{
			Percentage:   19,
			ProductTypes: []string{"E-Book"},
			Countries:    []string{"USA"},
		}},
	}

	item1 := &TestItem{
		price:    2900,
		itemType: "Book",
		items: []Item{&TestItem{
			price:    1900,
			itemType: "Book",
		}, &TestItem{
			price:    1000,
			itemType: "E-Book",
		}},
	}
	item2 := &TestItem{
		price:    3490,
		itemType: "Book",
		items: []Item{&TestItem{
			price:    2300,
			itemType: "Book",
		}, &TestItem{
			price:    1190,
			itemType: "E-Book",
		}},
	}
	params := PriceParameters{"USA", "USD", nil, []Item{item1, item2}}
	price := CalculatePrice(settings, nil, params, testLogger)

	assert.Equal(t, uint64(5766), price.Subtotal)
	assert.Equal(t, uint64(625), price.Taxes)
	assert.Equal(t, uint64(0), price.Discount)
	assert.Equal(t, int64(6391), price.Total)
}
