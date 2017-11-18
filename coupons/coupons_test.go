package coupons

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netlify/gocommerce/conf"
)

func TestRelativeURL(t *testing.T) {
	var callCount int
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		assert.Equal(t, "/this/is/where/the/coupons/are", r.URL.Path)
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "kitten", user)
		assert.Equal(t, "catnip4life", pass)

		rsp := map[string]interface{}{
			"coupons": map[string]interface{}{
				"meow": map[string]interface{}{
					"code": "catnip",
				},
				"magic": map[string]interface{}{},
			},
		}
		data, err := json.Marshal(&rsp)
		require.NoError(t, err)
		w.Write(data)
	}))

	defer svr.Close()
	c := &conf.Configuration{
		SiteURL: svr.URL,
	}
	c.Coupons.URL = "this/is/where/the/coupons/are"
	c.Coupons.User = "kitten"
	c.Coupons.Password = "catnip4life"

	cacheFace, err := NewCouponCacheFromURL(c)
	require.NoError(t, err)
	require.NotNil(t, cacheFace)

	cache, ok := cacheFace.(*couponCacheFromURL)
	require.True(t, ok)

	assert.Equal(t, svr.URL+"/this/is/where/the/coupons/are", cache.url)

	coupons, err := cache.List()
	require.NoError(t, err)

	require.Equal(t, 2, len(coupons))
	meow := coupons["meow"]
	assert.Equal(t, "catnip", meow.Code)
	assert.Equal(t, callCount, 1)
	magic := coupons["magic"]
	assert.Equal(t, "magic", magic.Code)

	// make sure this is cached
	_, err = cache.List()
	require.NoError(t, err)
	assert.Equal(t, callCount, 1)
}

func TestExplicitLookup(t *testing.T) {
	var callCount int
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		assert.Equal(t, "/this/is/where/the/coupons/are", r.URL.Path)
		rsp := map[string]interface{}{
			"coupons": map[string]interface{}{
				"meow": map[string]interface{}{},
			},
		}
		data, err := json.Marshal(&rsp)
		require.NoError(t, err)
		w.Write(data)
	}))

	defer svr.Close()
	c := &conf.Configuration{
		SiteURL: svr.URL,
	}
	c.Coupons.URL = "this/is/where/the/coupons/are"
	cache := newCache(t, c)
	assert.Equal(t, svr.URL+"/this/is/where/the/coupons/are", cache.url)

	coupon, err := cache.Lookup("meow")
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	assert.Equal(t, "meow", coupon.Code)

	coupon, err = cache.Lookup("dne")
	assert.Error(t, err)
	assert.IsType(t, new(CouponNotFound), err)
	assert.Nil(t, coupon)
	assert.Equal(t, 1, callCount)
}

func TestCacheExpiration(t *testing.T) {
	var callCount int
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		assert.Equal(t, "/this/is/where/the/coupons/are", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	c := &conf.Configuration{
		SiteURL: "this is garbage",
	}
	// note that this is an absolute path, it is what is used
	c.Coupons.URL = svr.URL + "/this/is/where/the/coupons/are"
	cache := newCache(t, c)
	assert.Equal(t, svr.URL+"/this/is/where/the/coupons/are", cache.url)

	coupon, err := cache.Lookup("meow")
	assert.Error(t, err)
	assert.IsType(t, new(CouponNotFound), err, err.Error())
	assert.Nil(t, coupon)
	assert.Equal(t, 1, callCount)

	// pretend we made this request a _while_ ago
	cache.lastFetch = time.Now().Add(-2 * cacheTime)

	coupon, err = cache.Lookup("meow")
	assert.Error(t, err)
	assert.IsType(t, new(CouponNotFound), err, err.Error())
	assert.Nil(t, coupon)
	assert.Equal(t, 2, callCount)
}

func TestMalformedResponse(t *testing.T) {
	var callCount int
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("this is not json"))
	}))
	defer svr.Close()

	c := &conf.Configuration{
		SiteURL: svr.URL,
	}
	c.Coupons.URL = "/this/is/where/the/coupons/are"
	cache := newCache(t, c)
	assert.Equal(t, svr.URL+"/this/is/where/the/coupons/are", cache.url)

	coupon, err := cache.Lookup("meow")
	assert.Error(t, err)
	assert.Nil(t, coupon)
	assert.Equal(t, 1, callCount)
}

func newCache(t *testing.T, c *conf.Configuration) *couponCacheFromURL {
	cacheFace, err := NewCouponCacheFromURL(c)
	require.NoError(t, err)
	require.NotNil(t, cacheFace)
	cache, ok := cacheFace.(*couponCacheFromURL)
	require.True(t, ok)
	return cache
}
