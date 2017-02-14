package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/guregu/kami"
	"github.com/netlify/netlify-commerce/conf"
	"github.com/netlify/netlify-commerce/models"
	"github.com/stretchr/testify/assert"
)

func TestNoCoupons(t *testing.T) {
	db, config := db(t)

	ctx := testContext(nil, config, false)
	ctx = kami.SetParam(ctx, "code", "coupon-code")

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "https://example.org", nil)

	NewAPI(config, db, nil, nil, nil).CouponView(ctx, recorder, req)
	validateError(t, 404, recorder)
}

func TestSimpleCouponLookup(t *testing.T) {
	db, config := db(t)

	startTestCouponURLs(config)

	ctx := testContext(nil, config, false)
	ctx = kami.SetParam(ctx, "code", "coupon-code")
	ctx = context.WithValue(ctx, couponsKey, NewCouponCacheFromUrl(config))

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "https://example.org", nil)

	NewAPI(config, db, nil, nil, nil).CouponView(ctx, recorder, req)
	coupon := &models.Coupon{}
	extractPayload(t, 200, recorder, coupon)
	assert.Equal(t, 15, coupon.Percentage, "Expected coupon percetage to be 15")
	assert.Equal(t, "coupon-code", coupon.Code, "Expected coupon code to be 'coupon-code'")
}

func TestApplyingACoupon(t *testing.T) {

}

func startTestCouponURLs(config *conf.Configuration) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{
      "coupons": {
        "coupon-code": {
          "percentage": 15
        }
      }
    }`)
	}))

	config.Coupons.URL = ts.URL
}
