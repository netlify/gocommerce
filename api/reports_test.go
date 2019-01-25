package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSalesReport(t *testing.T) {
	t.Run("AllTime", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testAdminToken("admin-yo", "admin@wayneindustries.com")
		recorder := test.TestEndpoint(http.MethodGet, "/reports/sales", nil, token)

		report := []salesRow{}
		extractPayload(t, http.StatusOK, recorder, &report)
		assert.Len(t, report, 1)
		row := report[0]
		assert.Equal(t, uint64(79), row.Total)
		assert.Equal(t, uint64(79), row.SubTotal)
		assert.Equal(t, uint64(0), row.Taxes)
		assert.Equal(t, "USD", row.Currency)
		assert.Equal(t, uint64(2), row.Orders)
	})
}

func TestProductsReport(t *testing.T) {
	test := NewRouteTest(t)
	token := testAdminToken("admin-yo", "admin@wayneindustries.com")
	recorder := test.TestEndpoint(http.MethodGet, "/reports/products", nil, token)

	report := []productsRow{}
	extractPayload(t, http.StatusOK, recorder, &report)
	assert.Len(t, report, 3)
	prod1 := report[0]
	assert.Equal(t, "234-fancy-belts", prod1.Sku)
	assert.Equal(t, uint64(45), prod1.Total)
	prod2 := report[1]
	assert.Equal(t, "123-i-can-fly-456", prod2.Sku)
	assert.Equal(t, uint64(24), prod2.Total)
	prod3 := report[2]
	assert.Equal(t, "456-i-rollover-all-things", prod3.Sku)
	assert.Equal(t, uint64(10), prod3.Total)
}
