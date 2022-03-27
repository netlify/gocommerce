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
	return 1.0
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
	assert.Equal(t, expected.Subtotal, actual.Subtotal, fmt.Sprintf("Expected subtotal to be %f, got %f", expected.Subtotal, actual.Subtotal))
	assert.Equal(t, expected.Taxes, actual.Taxes, fmt.Sprintf("Expected taxes to be %f, got %f", expected.Taxes, actual.Taxes))
	assert.Equal(t, expected.NetTotal, actual.NetTotal, fmt.Sprintf("Expected net total to be %f, got %f", expected.NetTotal, actual.NetTotal))
	assert.Equal(t, expected.Discount, actual.Discount, fmt.Sprintf("Expected discount to be %f, got %f", expected.Discount, actual.Discount))
	assert.Equal(t, expected.Total, actual.Total, fmt.Sprintf("Expected total to be %f, got %f", expected.Total, actual.Total))
	assert.Equal(t, float64(expected.NetTotal+expected.Taxes), expected.Total, "Your expected nettotal and taxes should add up to the expected total. Check your test!")
	assert.Equal(t, float64(actual.NetTotal+actual.Taxes), actual.Total, "Expected nettotal and taxes to add up to total")
}

func TestNoItems(t *testing.T) {
	params := PriceParameters{"USA", "USD", nil, nil}
	price := CalculatePrice(nil, nil, params, testLogger)
	validatePrice(t, price, Price{
		Subtotal: 0.0,
		Discount: 0.0,
		NetTotal: 0.0,
		Taxes:    0.0,
		Total:    0.0,
	})
}

func TestNoTaxes(t *testing.T) {
	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100.0, itemType: "test"}}}
	price := CalculatePrice(nil, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 100.0,
		Discount: 0.0,
		NetTotal: 100.0,
		Taxes:    0.0,
		Total:    100.0,
	})
}

func TestFixedVAT(t *testing.T) {
	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100.0, itemType: "test", vat: 9}}}
	price := CalculatePrice(nil, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 100.0,
		Discount: 0.0,
		NetTotal: 100.0,
		Taxes:    9.0,
		Total:    109.0,
	})
}

func TestFixedVATWhenPricesIncludeTaxes(t *testing.T) {
	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100.0, itemType: "test", vat: 9}}}
	price := CalculatePrice(&Settings{PricesIncludeTaxes: true}, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 92.0,
		Discount: 0.0,
		NetTotal: 92.0,
		Taxes:    8.0,
		Total:    100.0,
	})
}

func TestCountryBasedVAT(t *testing.T) {
	settings := &Settings{
		Taxes: []*Tax{&Tax{
			Percentage:   21.0,
			ProductTypes: []string{"test"},
			Countries:    []string{"USA"},
		}},
	}

	params := PriceParameters{"USA", "USD", nil, []Item{&TestItem{price: 100.0, itemType: "test"}}}
	price := CalculatePrice(settings, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 100.0,
		Discount: 0.0,
		NetTotal: 100.0,
		Taxes:    21.0,
		Total:    121.0,
	})
}

func TestCouponWithNoTaxes(t *testing.T) {
	coupon := &TestCoupon{itemType: "test", percentage: 10.0}
	params := PriceParameters{"USA", "USD", coupon, []Item{&TestItem{price: 100.0, itemType: "test"}}}
	price := CalculatePrice(nil, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 100.0,
		Discount: 10.0,
		NetTotal: 90.0,
		Taxes:    0.0,
		Total:    90.0,
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
		Subtotal: 183,
		Discount: 20,
		NetTotal: 165,
		Taxes:    15,
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

	t.Run("Single items", func(t *testing.T) {
		settings.Taxes = []*Tax{&Tax{
			Percentage:   7,
			ProductTypes: []string{"Book"},
			Countries:    []string{"USA"},
		}, &Tax{
			Percentage:   21,
			ProductTypes: []string{"E-Book"},
			Countries:    []string{"USA"},
		}}

		params := PriceParameters{"USA", "USD", nil, []Item{item1}}
		price := CalculatePrice(settings, nil, params, testLogger)

		validatePrice(t, price, Price{
			Subtotal: 2602,
			Discount: 0,
			NetTotal: 2602,
			Taxes:    298,
			Total:    2900,
		})
	})

	t.Run("Two items", func(t *testing.T) {
		settings.Taxes = []*Tax{&Tax{
			Percentage:   7,
			ProductTypes: []string{"Book"},
			Countries:    []string{"USA"},
		}, &Tax{
			Percentage:   19,
			ProductTypes: []string{"E-Book"},
			Countries:    []string{"USA"},
		}}

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
			Taxes:    624,
			Total:    6390,
		})
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
		price:    3900,
		itemType: "book",
		items: []Item{&TestItem{
			price:    2900,
			itemType: "book",
		}, &TestItem{
			price:    1000,
			itemType: "ebook",
		}},
	}

	coupon := &TestCoupon{itemType: "book", percentage: 25}
	params := PriceParameters{"Germany", "EUR", coupon, []Item{item}}
	price := CalculatePrice(settings, nil, params, testLogger)

	validatePrice(t, price, Price{
		Subtotal: 3550,
		Discount: 975,
		NetTotal: 2663,
		Taxes:    262,
		Total:    2925,
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
		price:    3900,
		itemType: "book",
		items: []Item{&TestItem{
			price:    2900,
			itemType: "book",
		}, &TestItem{
			price:    1000,
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
		Subtotal: 3550,
		Discount: 1000,
		NetTotal: 2640,
		Taxes:    260,
		Total:    2900,
	})
}
