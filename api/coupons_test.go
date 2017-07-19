package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/netlify/gocommerce/models"
	"github.com/stretchr/testify/assert"
)

func TestCouponView(t *testing.T) {
	t.Run("NotFound", func(t *testing.T) {
		test := NewRouteTest(t)
		recorder := test.TestEndpoint(http.MethodGet, "/coupons/coupon-code", nil, nil)
		validateError(t, http.StatusNotFound, recorder)
	})
	t.Run("Simple", func(t *testing.T) {
		test := NewRouteTest(t)
		server := startTestCouponURLs()
		defer server.Close()
		test.Config.Coupons.URL = server.URL

		recorder := test.TestEndpoint(http.MethodGet, "/coupons/coupon-code", nil, nil)
		coupon := &models.Coupon{}
		extractPayload(t, http.StatusOK, recorder, coupon)
		assert.Equal(t, uint64(15), coupon.Percentage, "Expected coupon percetage to be 15")
		assert.Equal(t, "coupon-code", coupon.Code, "Expected coupon code to be 'coupon-code'")
	})
}

func startTestCouponURLs() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{
      "coupons": {
        "coupon-code": {
          "percentage": 15
        }
      }
    }`)
	}))
}
