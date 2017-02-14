package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/guregu/kami"
	"github.com/netlify/netlify-commerce/conf"
	"github.com/netlify/netlify-commerce/models"
	"golang.org/x/net/context"
)

const CacheTime = 1 * time.Minute

type CouponApplyParams struct {
	OrderID string `json:"order_id"`
	JWT     string `json:"jwt"`
}

type CouponNotFound struct{}

func (CouponNotFound) Error() string {
	return "Coupon not found"
}

type CouponCache interface {
	Lookup(string) (*models.Coupon, error)
}

type couponsResponse struct {
	Coupons map[string]*models.Coupon `json:"coupons"`
}

type CouponCacheFromURL struct {
	url       string
	user      string
	password  string
	lastFetch time.Time
	coupons   map[string]*models.Coupon
	mutex     sync.Mutex
	client    *http.Client
}

func NewCouponCacheFromUrl(config *conf.Configuration) CouponCache {
	return &CouponCacheFromURL{
		url:      config.Coupons.URL,
		user:     config.Coupons.User,
		password: config.Coupons.Password,
		coupons:  map[string]*models.Coupon{},
		client:   &http.Client{},
	}
}

func (c *CouponCacheFromURL) Lookup(code string) (*models.Coupon, error) {
	if c.coupons != nil && time.Now().Before(c.lastFetch.Add(CacheTime)) {
		coupon, ok := c.coupons[code]
		if ok {
			return coupon, nil
		}
		return nil, &CouponNotFound{}
	}
	req, err := http.NewRequest("GET", c.url, nil)
	if err != nil {
		return nil, err
	}
	if c.user != "" {
		req.SetBasicAuth(c.user, c.password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Coupon URL returned %v", resp.StatusCode)
	}

	couponsResponse := &couponsResponse{}
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(couponsResponse); err != nil {
		return nil, err
	}
	for key, coupon := range couponsResponse.Coupons {
		coupon.Code = key
	}

	c.mutex.Lock()
	c.coupons = couponsResponse.Coupons
	c.mutex.Unlock()

	coupon, ok := c.coupons[code]
	if ok {
		return coupon, nil
	}
	return nil, &CouponNotFound{}
}

func (a *API) lookupCoupon(ctx context.Context, w http.ResponseWriter, code string) (*models.Coupon, error) {
	coupons := getCoupons(ctx)
	if coupons == nil {
		notFoundError(w, "No coupons available")
		return nil, errors.New("No coupons available")
	}

	coupon, err := coupons.Lookup(code)
	if err != nil {
		switch v := err.(type) {
		case CouponNotFound:
			notFoundError(w, v.Error())
		default:
			internalServerError(w, "Error fetching coupon: %v", err)
		}
		return nil, err
	}

	return coupon, nil
}

func (a *API) CouponView(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	code := kami.Param(ctx, "code")
	coupon, err := a.lookupCoupon(ctx, w, code)
	if err != nil {
		a.log.WithError(err).Infof("error loading coupon %v", err)
	}

	sendJSON(w, 200, coupon)
}
