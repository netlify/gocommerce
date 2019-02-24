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
	price    float64
	itemType string
	vat      float64
	items    []Item
	quantity float64
}

func (t *TestItem) ProductSku() string {
	return t.sku
}

func (t *TestItem) PriceInLowestUnit() float64 {
	return t.price
}

func (t *TestItem) ProductType() string {
	return t.itemType
}

func (t *TestItem) FixedVAT() float64 {
	return t.vat
}

func (t *TestItem) TaxableItems() []Item {
	return t.items
}

func (t *TestItem) GetQuantity() float64 {
	if t.quantity > 0 {
		return t.quantity
	}
	return 1
}

type TestCoupon struct {
	itemSku    string
	itemType   string
	moreThan   float64
	percentage float64
	fixed      float64
}

func (c *TestCoupon) ValidForType(productType string) bool {
	return c.itemType == productType
}

func (c *TestCoupon) ValidForProduct(productSku string) bool {
	return c.itemSku == productSku
}

func (c *TestCoupon) ValidForPrice(currency string, price float64) bool {
	return c.moreThan == 0 || price > c.moreThan
}

func (c *TestCoupon) PercentageDiscount() float64 {
	return c.percentage
}

func (c *TestCoupon) FixedDiscount(currency string) float64 {
	return c.fixed
}

func validatePrice(t *testing.T, actual Price, expected Price) {
	assert.InDelta(t, expected.Subtotal, actual.Subtotal, 1.00, fmt.Sprintf("Expected subtotal to be %f, got %f", expected.Subtotal, actual.Subtotal))
	assert.InDelta(t, expected.Taxes, actual.Taxes, 1.00, fmt.Sprintf("Expected taxes to be %f, got %f", expected.Taxes, actual.Taxes))
	assert.InDelta(t, expected.NetTotal, actual.NetTotal, 1.00, fmt.Sprintf("Expected net total to be %f, got %f", expected.NetTotal, actual.NetTotal))
	assert.InDelta(t, expected.Discount, actual.Discount, 1.00, fmt.Sprintf("Expected discount to be %f, got %f", expected.Discount, actual.Discount))
	assert.InDelta(t, expected.Total, actual.Total, 1.00, fmt.Sprintf("Expected total to be %f, got %f", expected.Total, actual.Total))
	assert.InDelta(t, float64(expected.NetTotal+expected.Taxes), expected.Total, 1.00, "Your expected nettotal and taxes should add up to the expected total. Check your test!")
	assert.InDelta(t, float64(actual.NetTotal+actual.Taxes), actual.Total, 1.00, "Expected nettotal and taxes to add up to total")
}

func TestNoItems(t *testing.T) {
	params := PriceParameters{"USA", "USD", nil, nil}
	price := CalculatePrice(nil, nil, params, testLogger)
	validatePrice(t, price, Price{
		Subtotal: 0.00,
		Discount: 0.00,
		NetTotal: 0.00,
		Taxes:    0.00,
		Total:    0.00,
	})
}

func TestNoTaxes(t *testing.T) {
	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100.00, itemType: "test"}}}
	price := CalculatePrice(nil, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 100.00,
		Discount: 0.00,
		NetTotal: 100.00,
		Taxes:    0.00,
		Total:    100.00,
	})
}

func TestFixedVAT(t *testing.T) {
	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100.00, itemType: "test", vat: 9.00}}}
	price := CalculatePrice(nil, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 100.00,
		Discount: 0.00,
		NetTotal: 100.00,
		Taxes:    9.00,
		Total:    109.00,
	})
}

func TestFixedVATWhenPricesIncludeTaxes(t *testing.T) {
	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100.00, itemType: "test", vat: 9.00}}}
	price := CalculatePrice(&Settings{PricesIncludeTaxes: true}, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 92.00,
		Discount: 0.00,
		NetTotal: 92.00,
		Taxes:    8.00,
		Total:    100.00,
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

	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100.00, itemType: "test"}}}
	price := CalculatePrice(settings, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 100.00,
		Discount: 0.00,
		NetTotal: 100.00,
		Taxes:    21.00,
		Total:    121.00,
	})
}

func TestCouponWithNoTaxes(t *testing.T) {
	coupon := &TestCoupon{itemType: "test", percentage: 10}
	params := PriceParameters{"USA", "USD", coupon, []Item{&TestItem{price: 100.00, itemType: "test"}}}
	price := CalculatePrice(nil, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 100.00,
		Discount: 10.00,
		NetTotal: 90.00,
		Taxes:    0.00,
		Total:    90.00,
	})
}

func TestCouponWithVAT(t *testing.T) {
	coupon := &TestCoupon{itemType: "test", percentage: 10}
	params := PriceParameters{"USA", "USD", coupon, []Item{&TestItem{price: 100.00, itemType: "test", vat: 10.00}}}
	price := CalculatePrice(nil, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 100.00,
		Discount: 10.00,
		NetTotal: 90.00,
		Taxes:    9.00,
		Total:    99.00,
	})
}

func TestCouponWithVATWhenPRiceIncludeTaxes(t *testing.T) {
	coupon := &TestCoupon{itemType: "test", percentage: 10}
	settings := &Settings{PricesIncludeTaxes: true}
	params := PriceParameters{"USA", "USD", coupon, []Item{&TestItem{price: 100.00, itemType: "test", vat: 9.00}}}
	price := CalculatePrice(settings, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 92.00,
		Discount: 10.00,
		NetTotal: 83.00,
		Taxes:    7.00,
		Total:    90.00,
	})
}

func TestCouponWithVATWhenPRiceIncludeTaxesWithQuantity(t *testing.T) {
	coupon := &TestCoupon{itemType: "test", percentage: 10}
	settings := &Settings{PricesIncludeTaxes: true}
	params := PriceParameters{"USA", "USD", coupon, []Item{&TestItem{quantity: 2.00, price: 100.00, itemType: "test", vat: 9.00}}}
	price := CalculatePrice(settings, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 183.00,
		Discount: 20.00,
		NetTotal: 165.00,
		Taxes:    15.00,
		Total:    180.00,
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
		price:    100.00,
		itemType: "book",
		items: []Item{&TestItem{
			price:    80.00,
			itemType: "book",
		}, &TestItem{
			price:    20.00,
			itemType: "ebook",
		}},
	}
	params := PriceParameters{"DE", "USD", nil, []Item{item}}
	price := CalculatePrice(settings, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 100.00,
		Discount: 0.00,
		NetTotal: 100.00,
		Taxes:    10.00,
		Total:    110.00,
	})
}

func TestMemberDiscounts(t *testing.T) {
	settings := &Settings{PricesIncludeTaxes: true, MemberDiscounts: []*MemberDiscount{&MemberDiscount{
		Claims:     map[string]string{"app_metadata.plan": "member"},
		Percentage: 10,
	}}}
	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100.00, itemType: "test", vat: 9.00}}}
	price := CalculatePrice(settings, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 92.00,
		Discount: 0.00,
		NetTotal: 92.00,
		Taxes:    8.00,
		Total:    100.00,
	})

	claims := map[string]interface{}{}
	require.NoError(t, json.Unmarshal([]byte(`{"app_metadata": {"plan": "member"}}`), &claims))

	params = PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100.00, itemType: "test", vat: 9.00}}}
	price = CalculatePrice(settings, claims, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 92.00,
		Discount: 10.00,
		NetTotal: 83.00,
		Taxes:    7.00,
		Total:    90.00,
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

	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100.00, itemType: "test", vat: 9.00}}}
	price := CalculatePrice(settings, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 92.00,
		Discount: 0.00,
		NetTotal: 92.00,
		Taxes:    8.00,
		Total:    100.00,
	})

	claims := map[string]interface{}{}
	require.NoError(t, json.Unmarshal([]byte(`{"app_metadata": {"plan": "member"}}`), &claims))

	params = PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100.00, itemType: "test", vat: 9.00}}}
	price = CalculatePrice(settings, claims, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 92.00,
		Discount: 10.00,
		NetTotal: 83.00,
		Taxes:    7.00,
		Total:    90.00,
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
		quantity: 1.00,
		price:    3490.00,
	}

	params := PriceParameters{"USA", "USD", nil, []Item{item}}
	price := CalculatePrice(&settings, nil, params, testLogger)
	assert.Equal(t, 3490.00, float64(price.Total))

	claims := map[string]interface{}{
		"app_metadata": map[string]interface{}{
			"subscription": map[string]interface{}{
				"plan": "member",
			},
		},
	}
	price = CalculatePrice(&settings, claims, params, testLogger)
	assert.Equal(t, float64(0), price.Total)
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
		price:    2900.00,
		itemType: "Book",
		items: []Item{&TestItem{
			price:    1900.00,
			itemType: "Book",
		}, &TestItem{
			price:    1000.00,
			itemType: "E-Book",
		}},
	}
	item2 := &TestItem{
		price:    3490.00,
		itemType: "Book",
		items: []Item{&TestItem{
			price:    2300.00,
			itemType: "Book",
		}, &TestItem{
			price:    1190.00,
			itemType: "E-Book",
		}},
	}
	params := PriceParameters{"USA", "USD", nil, []Item{item1, item2}}
	price := CalculatePrice(settings, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 5766.00,
		Discount: 0.00,
		NetTotal: 5766.00,
		Taxes:    625.00,
		Total:    6391.00,
	})
}

func TestRealWorldRelativeDiscountWithTaxes(t *testing.T) {
	settings := &Settings{
		PricesIncludeTaxes: true,
		Taxes: []*Tax{&Tax{
			Percentage:   7,
			ProductTypes: []string{"book"},
			Countries:    []string{"Germany"},
		}, &Tax{
			Percentage:   19,
			ProductTypes: []string{"ebook"},
			Countries:    []string{"Germany"},
		}},
	}

	item := &TestItem{
		price:    3900.00,
		itemType: "book",
		items: []Item{&TestItem{
			price:    2900.00,
			itemType: "book",
		}, &TestItem{
			price:    1000.00,
			itemType: "ebook",
		}},
	}

	coupon := &TestCoupon{itemType: "book", percentage: 25}
	params := PriceParameters{"Germany", "EUR", coupon, []Item{item}}
	price := CalculatePrice(settings, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 3550.00,
		Discount: 975.00,
		NetTotal: 2663.00,
		Taxes:    262.00,
		Total:    2925.00,
	})
}

func TestRealWorldFixedDiscountWithTaxes(t *testing.T) {
	settings := &Settings{
		PricesIncludeTaxes: true,
		Taxes: []*Tax{&Tax{
			Percentage:   7,
			ProductTypes: []string{"book"},
			Countries:    []string{"Germany"},
		}, &Tax{
			Percentage:   19,
			ProductTypes: []string{"ebook"},
			Countries:    []string{"Germany"},
		}},
		MemberDiscounts: []*MemberDiscount{&MemberDiscount{
			Claims: map[string]string{
				"app_metadata.subscription.plan": "member",
			},
			FixedAmount: []*FixedMemberDiscount{&FixedMemberDiscount{
				Amount:   "10.00",
				Currency: "EUR",
			}},
			ProductTypes: []string{"book"},
		}},
	}

	item := &TestItem{
		price:    3900.00,
		itemType: "book",
		items: []Item{&TestItem{
			price:    2900.00,
			itemType: "book",
		}, &TestItem{
			price:    1000.00,
			itemType: "ebook",
		}},
	}

	claims := map[string]interface{}{
		"app_metadata": map[string]interface{}{
			"subscription": map[string]interface{}{
				"plan": "member",
			},
		},
	}
	params := PriceParameters{"Germany", "EUR", nil, []Item{item}}
	price := CalculatePrice(settings, claims, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 3550.00,
		Discount: 1000.00,
		NetTotal: 2640.00,
		Taxes:    260.00,
		Total:    2900.00,
	})
}
