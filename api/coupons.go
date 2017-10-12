package api

import (
	"net/http"

	"context"

	"github.com/go-chi/chi"
	gcontext "github.com/netlify/gocommerce/context"
	"github.com/netlify/gocommerce/coupons"
	"github.com/netlify/gocommerce/models"
)

func (a *API) lookupCoupon(ctx context.Context, w http.ResponseWriter, code string) (*models.Coupon, error) {
	couponCache := gcontext.GetCoupons(ctx)
	if couponCache == nil {
		return nil, notFoundError("No coupons available")
	}

	coupon, err := couponCache.Lookup(code)
	if err != nil {
		switch v := err.(type) {
		case coupons.CouponNotFound:
			return nil, notFoundError(v.Error())
		default:
			return nil, internalServerError("Error fetching coupon").WithInternalError(err)
		}
	}

	return coupon, nil
}

// CouponView returns information about a single coupon code.
func (a *API) CouponView(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := getLogEntry(r)
	code := chi.URLParam(r, "coupon_code")
	coupon, err := a.lookupCoupon(ctx, w, code)
	if err != nil {
		log.WithError(err).Infof("error loading coupon %v", err)
		return err
	}

	return sendJSON(w, http.StatusOK, coupon)
}

// CouponList returns all the coupons for the site. Requires admin permissions
func (a *API) CouponList(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := getLogEntry(r)

	couponCache := gcontext.GetCoupons(ctx)
	if couponCache == nil {
		return sendJSON(w, http.StatusOK, []string{})
	}

	coupons, err := couponCache.List()
	if err != nil {
		log.WithError(err).Errorf("Error loading coupons: %v", err)
		return internalServerError("Error fetching coupons: %v", err)
	}

	return sendJSON(w, http.StatusOK, coupons)
}
