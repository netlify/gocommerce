package coupons

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/models"
	"github.com/pkg/errors"
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
func NewCouponCacheFromURL(config *conf.Configuration) (Cache, error) {
	if config.Coupons.URL == "" {
		return nil, nil
	}

	url, err := url.Parse(config.Coupons.URL)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse Coupons URL")
	}

	if !url.IsAbs() {
		siteURL, err := url.Parse(config.SiteURL)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse Site URL")
		}
		url.Scheme = siteURL.Scheme
		url.Host = siteURL.Host
		url.User = siteURL.User
	}

	return &couponCacheFromURL{
		url:       url.String(),
		user:      config.Coupons.User,
		password:  config.Coupons.Password,
		coupons:   map[string]*models.Coupon{},
		client:    &http.Client{},
		lastFetch: time.Unix(0, 0),
	}, nil
}

func (c *couponCacheFromURL) load() error {
	req, err := http.NewRequest(http.MethodGet, c.url, nil)
	if err != nil {
		return err
	}
	if c.user != "" {
		req.SetBasicAuth(c.user, c.password)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "Failed to make request for coupon information")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Coupon URL returned %v", resp.StatusCode)
	}

	couponsResponse := &couponsResponse{}
	if resp.Body != nil && resp.Body != http.NoBody {
		defer resp.Body.Close()
		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(couponsResponse); err != nil {
			return errors.Wrap(err, "Failed to parse response.")
		}

		for key, coupon := range couponsResponse.Coupons {
			if coupon.Code == "" {
				coupon.Code = key
			}
		}
	}

	c.mutex.Lock()
	c.coupons = couponsResponse.Coupons
	c.lastFetch = time.Now()
	c.mutex.Unlock()

	return nil
}

func (c *couponCacheFromURL) Lookup(code string) (*models.Coupon, error) {
	if time.Now().After(c.lastFetch.Add(cacheTime)) {
		if err := c.load(); err != nil {
			return nil, err
		}
	}

	coupon, ok := c.coupons[code]
	if ok {
		return coupon, nil
	}
	return nil, &CouponNotFound{}
}

func (c *couponCacheFromURL) List() (map[string]*models.Coupon, error) {
	if time.Now().After(c.lastFetch.Add(cacheTime)) {
		if err := c.load(); err != nil {
			return nil, err
		}
	}

	return c.coupons, nil
}
