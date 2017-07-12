package api

import (
	"errors"
	"net/http"

	"context"

	"github.com/guregu/kami"
	gcontext "github.com/netlify/gocommerce/context"
	"github.com/netlify/gocommerce/coupons"
	"github.com/netlify/gocommerce/models"
)

func (a *API) lookupCoupon(ctx context.Context, w http.ResponseWriter, code string) (*models.Coupon, error) {
	couponCache := gcontext.GetCoupons(ctx)
	if couponCache == nil {
		notFoundError(w, "No coupons available")
		return nil, errors.New("No coupons available")
	}

	coupon, err := couponCache.Lookup(code)
	if err != nil {
		switch v := err.(type) {
		case coupons.CouponNotFound:
			notFoundError(w, v.Error())
		default:
			internalServerError(w, "Error fetching coupon: %v", err)
		}
		return nil, err
	}

	return coupon, nil
}

func (a *API) CouponView(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	log := gcontext.GetLogger(ctx)
	code := kami.Param(ctx, "code")
	coupon, err := a.lookupCoupon(ctx, w, code)
	if err != nil {
		log.WithError(err).Infof("error loading coupon %v", err)
		return
	}

	sendJSON(w, 200, coupon)
}
