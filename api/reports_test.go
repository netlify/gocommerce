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
