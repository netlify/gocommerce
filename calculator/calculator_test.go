package calculator

import (
	"encoding/json"
	"fmt"
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

func validatePrice(t *testing.T, actual Price, expected Price) {
	assert.Equal(t, expected.Subtotal, actual.Subtotal, fmt.Sprintf("Expected subtotal to be %d, got %d", expected.Subtotal, actual.Subtotal))
	assert.Equal(t, expected.Taxes, actual.Taxes, fmt.Sprintf("Expected taxes to be %d, got %d", expected.Taxes, actual.Taxes))
	assert.Equal(t, expected.NetTotal, actual.NetTotal, fmt.Sprintf("Expected net total to be %d, got %d", expected.NetTotal, actual.NetTotal))
	assert.Equal(t, expected.Discount, actual.Discount, fmt.Sprintf("Expected discount to be %d, got %d", expected.Discount, actual.Discount))
	assert.Equal(t, expected.Total, actual.Total, fmt.Sprintf("Expected total to be %d, got %d", expected.Total, actual.Total))
}

func TestNoItems(t *testing.T) {
	params := PriceParameters{"USA", "USD", nil, nil}
	price := CalculatePrice(nil, nil, params, testLogger)
	validatePrice(t, price, Price{
		Subtotal: 0,
		Discount: 0,
		NetTotal: 0,
		Taxes:    0,
		Total:    0,
	})
}

func TestNoTaxes(t *testing.T) {
	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100, itemType: "test"}}}
	price := CalculatePrice(nil, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 100,
		Discount: 0,
		NetTotal: 100,
		Taxes:    0,
		Total:    100,
	})
}

func TestFixedVAT(t *testing.T) {
	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100, itemType: "test", vat: 9}}}
	price := CalculatePrice(nil, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 100,
		Discount: 0,
		NetTotal: 100,
		Taxes:    9,
		Total:    109,
	})
}

func TestFixedVATWhenPricesIncludeTaxes(t *testing.T) {
	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100, itemType: "test", vat: 9}}}
	price := CalculatePrice(&Settings{PricesIncludeTaxes: true}, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 92,
		Discount: 0,
		NetTotal: 92,
		Taxes:    8,
		Total:    100,
	})
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

	validatePrice(t, price, Price{
		Subtotal: 100,
		Discount: 0,
		NetTotal: 100,
		Taxes:    21,
		Total:    121,
	})
}

func TestCouponWithNoTaxes(t *testing.T) {
	coupon := &TestCoupon{itemType: "test", percentage: 10}
	params := PriceParameters{"USA", "USD", coupon, []Item{&TestItem{price: 100, itemType: "test"}}}
	price := CalculatePrice(nil, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 100,
		Discount: 10,
		NetTotal: 90,
		Taxes:    0,
		Total:    90,
	})
}

func TestCouponWithVAT(t *testing.T) {
	coupon := &TestCoupon{itemType: "test", percentage: 10}
	params := PriceParameters{"USA", "USD", coupon, []Item{&TestItem{price: 100, itemType: "test", vat: 10}}}
	price := CalculatePrice(nil, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 100,
		Discount: 10,
		NetTotal: 90,
		Taxes:    9,
		Total:    99,
	})
}

func TestCouponWithVATWhenPRiceIncludeTaxes(t *testing.T) {
	coupon := &TestCoupon{itemType: "test", percentage: 10}
	settings := &Settings{PricesIncludeTaxes: true}
	params := PriceParameters{"USA", "USD", coupon, []Item{&TestItem{price: 100, itemType: "test", vat: 9}}}
	price := CalculatePrice(settings, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 92,
		Discount: 10,
		NetTotal: 83,
		Taxes:    7,
		Total:    90,
	})
}

func TestCouponWithVATWhenPRiceIncludeTaxesWithQuantity(t *testing.T) {
	coupon := &TestCoupon{itemType: "test", percentage: 10}
	settings := &Settings{PricesIncludeTaxes: true}
	params := PriceParameters{"USA", "USD", coupon, []Item{&TestItem{quantity: 2, price: 100, itemType: "test", vat: 9}}}
	price := CalculatePrice(settings, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 184,
		Discount: 20,
		NetTotal: 166,
		Taxes:    14,
		Total:    180,
	})
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

	validatePrice(t, price, Price{
		Subtotal: 100,
		Discount: 0,
		NetTotal: 100,
		Taxes:    10,
		Total:    110,
	})
}

func TestMemberDiscounts(t *testing.T) {
	settings := &Settings{PricesIncludeTaxes: true, MemberDiscounts: []*MemberDiscount{&MemberDiscount{
		Claims:     map[string]string{"app_metadata.plan": "member"},
		Percentage: 10,
	}}}
	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100, itemType: "test", vat: 9}}}
	price := CalculatePrice(settings, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 92,
		Discount: 0,
		NetTotal: 92,
		Taxes:    8,
		Total:    100,
	})

	claims := map[string]interface{}{}
	require.NoError(t, json.Unmarshal([]byte(`{"app_metadata": {"plan": "member"}}`), &claims))

	params = PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100, itemType: "test", vat: 9}}}
	price = CalculatePrice(settings, claims, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 92,
		Discount: 10,
		NetTotal: 83,
		Taxes:    7,
		Total:    90,
	})
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

	validatePrice(t, price, Price{
		Subtotal: 92,
		Discount: 0,
		NetTotal: 92,
		Taxes:    8,
		Total:    100,
	})

	claims := map[string]interface{}{}
	require.NoError(t, json.Unmarshal([]byte(`{"app_metadata": {"plan": "member"}}`), &claims))

	params = PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100, itemType: "test", vat: 9}}}
	price = CalculatePrice(settings, claims, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 92,
		Discount: 10,
		NetTotal: 83,
		Taxes:    7,
		Total:    90,
	})
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

	validatePrice(t, price, Price{
		Subtotal: 5766,
		Discount: 0,
		NetTotal: 5766,
		Taxes:    625,
		Total:    6391,
	})
}
