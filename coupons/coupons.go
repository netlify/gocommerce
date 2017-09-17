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

const cacheTime = 1 * time.Minute

// Cache is an interface for how to lookup a coupon based upon the code.
type Cache interface {
	Lookup(string) (*models.Coupon, error)
	List() (map[string]*models.Coupon, error)
}

// CouponNotFound is an error when a coupon could not be found.
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

// NewCouponCacheFromURL creates a coupon cache using the provided configuration.
func NewCouponCacheFromURL(config *conf.Configuration) Cache {
	if config.Coupons.URL == "" {
		return nil
	}

	return &couponCacheFromURL{
		url:      config.Coupons.URL,
		user:     config.Coupons.User,
		password: config.Coupons.Password,
		coupons:  map[string]*models.Coupon{},
		client:   &http.Client{},
	}
}

func (c *couponCacheFromURL) load() error {
	req, err := http.NewRequest("GET", c.url, nil)
	if err != nil {
		return err
	}
	if c.user != "" {
		req.SetBasicAuth(c.user, c.password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Coupon URL returned %v", resp.StatusCode)
	}

	couponsResponse := &couponsResponse{}
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(couponsResponse); err != nil {
		return err
	}
	for key, coupon := range couponsResponse.Coupons {
		coupon.Code = key
	}

	c.mutex.Lock()
	c.coupons = couponsResponse.Coupons
	c.mutex.Unlock()

	return nil
}

func (c *couponCacheFromURL) Lookup(code string) (*models.Coupon, error) {
	if c.coupons != nil && time.Now().Before(c.lastFetch.Add(cacheTime)) {
		coupon, ok := c.coupons[code]
		if ok {
			return coupon, nil
		}
		return nil, &CouponNotFound{}
	}
	if err := c.load(); err != nil {
		return nil, err
	}

	coupon, ok := c.coupons[code]
	if ok {
		return coupon, nil
	}
	return nil, &CouponNotFound{}
}

func (c *couponCacheFromURL) List() (map[string]*models.Coupon, error) {
	if c.coupons == nil {
		if err := c.load(); err != nil {
			return nil, err
		}
	}

	return c.coupons, nil
}
