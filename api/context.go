package api

import (
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/dgrijalva/jwt-go"
	"golang.org/x/net/context"

	"github.com/netlify/gocommerce/conf"
)

const (
	tokenKey     = "jwt"
	configKey    = "config"
	couponsKey   = "coupons"
	loggerKey    = "logger"
	requestIDKey = "request_id"
	startKey     = "request_start_time"
	adminFlagKey = "is_admin"
	payerKey     = "payer_interface"
)

type ChargerType string

const PaypalChargerType ChargerType = "paypal"
const StripeChargerType ChargerType = "stripe"

func withStartTime(ctx context.Context, when time.Time) context.Context {
	return context.WithValue(ctx, startKey, &when)
}

func withLogger(ctx context.Context, l *logrus.Entry) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

func withConfig(ctx context.Context, config *conf.Configuration) context.Context {
	return context.WithValue(ctx, configKey, config)
}

func withCoupons(ctx context.Context, config *conf.Configuration) context.Context {
	return context.WithValue(ctx, couponsKey, NewCouponCacheFromUrl(config))
}

func withToken(ctx context.Context, token *jwt.Token) context.Context {
	return context.WithValue(ctx, tokenKey, token)
}

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func withPayer(ctx context.Context, chType ChargerType, pp paymentProvider) context.Context {
	return context.WithValue(ctx, payerKey+chType, pp)
}

func getCharger(ctx context.Context, chType ChargerType) paymentProvider {
	obj := ctx.Value(payerKey + chType)
	if obj == nil {
		return nil
	}

	return obj.(paymentProvider)
}

func getRequestID(ctx context.Context) string {
	obj := ctx.Value(requestIDKey)
	if obj == nil {
		return ""
	}

	return obj.(string)
}

func getStartTime(ctx context.Context) *time.Time {
	obj := ctx.Value(startKey)
	if obj == nil {
		return nil
	}

	return obj.(*time.Time)
}

func getConfig(ctx context.Context) *conf.Configuration {
	obj := ctx.Value(configKey)
	if obj == nil {
		return nil
	}

	return obj.(*conf.Configuration)
}

func getCoupons(ctx context.Context) CouponCache {
	obj := ctx.Value(couponsKey)
	if obj == nil {
		return nil
	}

	return obj.(CouponCache)
}

func getToken(ctx context.Context) *jwt.Token {
	obj := ctx.Value(tokenKey)
	if obj == nil {
		return nil
	}

	return obj.(*jwt.Token)
}

func getClaims(ctx context.Context) *JWTClaims {
	token := getToken(ctx)
	if token == nil {
		return nil
	}
	return token.Claims.(*JWTClaims)
}

func getClaimsAsMap(ctx context.Context) map[string]interface{} {
	token := getToken(ctx)
	if token == nil {
		return nil
	}
	config := getConfig(ctx)
	if config == nil {
		return nil
	}
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(token.Raw, &claims, func(token *jwt.Token) (interface{}, error) {
		if token.Header["alg"] != jwt.SigningMethodHS256.Name {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.JWT.Secret), nil
	})
	if err != nil {
		return nil
	}

	return map[string]interface{}(claims)
}

func withAdminFlag(ctx context.Context, isAdmin bool) context.Context {
	return context.WithValue(ctx, adminFlagKey, isAdmin)
}

func isAdmin(ctx context.Context) bool {
	obj := ctx.Value(adminFlagKey)
	if obj == nil {
		return false
	}
	return obj.(bool)
}

func getLogger(ctx context.Context) *logrus.Entry {
	obj := ctx.Value(loggerKey)
	if obj == nil {
		return logrus.NewEntry(logrus.StandardLogger())
	}
	return obj.(*logrus.Entry)
}
