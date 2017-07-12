package coupons

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/models"
)

const CacheTime = 1 * time.Minute

type Cache interface {
	Lookup(string) (*models.Coupon, error)
}

type CouponNotFound struct{}

func (CouponNotFound) Error() string {
	return "Coupon not found"
}

type couponsResponse struct {
	Coupons map[string]*models.Coupon `json:"coupons"`
}

type couponCacheFromURL struct {
	url       string
	user      string
	password  string
	lastFetch time.Time
	coupons   map[string]*models.Coupon
	mutex     sync.Mutex
	client    *http.Client
}

func NewCouponCacheFromUrl(config *conf.Configuration) Cache {
	return &couponCacheFromURL{
		url:      config.Coupons.URL,
		user:     config.Coupons.User,
		password: config.Coupons.Password,
		coupons:  map[string]*models.Coupon{},
		client:   &http.Client{},
	}
}

func (c *couponCacheFromURL) Lookup(code string) (*models.Coupon, error) {
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

	if resp.StatusCode != http.StatusOK {
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
