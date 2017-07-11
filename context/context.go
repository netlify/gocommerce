package context

import (
	"fmt"
	"time"

	"context"

	"github.com/Sirupsen/logrus"
	"github.com/dgrijalva/jwt-go"

	"github.com/netlify/gocommerce/assetstores"
	"github.com/netlify/gocommerce/claims"
	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/coupons"
	"github.com/netlify/gocommerce/mailer"
	"github.com/netlify/gocommerce/payments"
)

type contextKey string

func (c contextKey) String() string {
	return "api context key " + string(c)
}

const (
	tokenKey           = contextKey("jwt")
	configKey          = contextKey("config")
	couponsKey         = contextKey("coupons")
	loggerKey          = contextKey("logger")
	requestIDKey       = contextKey("request_id")
	startKey           = contextKey("request_start_time")
	adminFlagKey       = contextKey("is_admin")
	mailerKey          = contextKey("mailer")
	assetStoreKey      = contextKey("asset_store")
	paymentProviderKey = contextKey("payment-provider")
)

func WithStartTime(ctx context.Context, when time.Time) context.Context {
	return context.WithValue(ctx, startKey, &when)
}
func GetStartTime(ctx context.Context) *time.Time {
	obj := ctx.Value(startKey)
	if obj == nil {
		return nil
	}

	return obj.(*time.Time)
}

func WithLogger(ctx context.Context, l *logrus.Entry) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}
func GetLogger(ctx context.Context) *logrus.Entry {
	obj := ctx.Value(loggerKey)
	if obj == nil {
		return logrus.NewEntry(logrus.StandardLogger())
	}
	return obj.(*logrus.Entry)
}

func WithConfig(ctx context.Context, config *conf.Configuration) context.Context {
	return context.WithValue(ctx, configKey, config)
}
func GetConfig(ctx context.Context) *conf.Configuration {
	obj := ctx.Value(configKey)
	if obj == nil {
		return nil
	}

	return obj.(*conf.Configuration)
}

func WithCoupons(ctx context.Context, config *conf.Configuration) context.Context {
	return context.WithValue(ctx, couponsKey, coupons.NewCouponCacheFromUrl(config))
}
func GetCoupons(ctx context.Context) coupons.Cache {
	obj := ctx.Value(couponsKey)
	if obj == nil {
		return nil
	}

	return obj.(coupons.Cache)
}

func WithToken(ctx context.Context, token *jwt.Token) context.Context {
	return context.WithValue(ctx, tokenKey, token)
}
func GetToken(ctx context.Context) *jwt.Token {
	obj := ctx.Value(tokenKey)
	if obj == nil {
		return nil
	}

	return obj.(*jwt.Token)
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}
func GetRequestID(ctx context.Context) string {
	obj := ctx.Value(requestIDKey)
	if obj == nil {
		return ""
	}

	return obj.(string)
}

func WithMailer(ctx context.Context, mailer mailer.Mailer) context.Context {
	return context.WithValue(ctx, mailerKey, mailer)
}
func GetMailer(ctx context.Context) mailer.Mailer {
	obj := ctx.Value(mailerKey)
	if obj == nil {
		return nil
	}
	return obj.(mailer.Mailer)
}

func WithAssetStore(ctx context.Context, store assetstores.Store) context.Context {
	return context.WithValue(ctx, assetStoreKey, store)
}
func GetAssetStore(ctx context.Context) assetstores.Store {
	obj := ctx.Value(assetStoreKey)
	if obj == nil {
		return nil
	}
	return obj.(assetstores.Store)
}

func WithPaymentProvider(ctx context.Context, prov payments.Provider) context.Context {
	return context.WithValue(ctx, paymentProviderKey, prov)
}

func GetPaymentProvider(ctx context.Context) payments.Provider {
	provs, _ := ctx.Value(paymentProviderKey).(payments.Provider)
	return provs
}

func GetClaims(ctx context.Context) *claims.JWTClaims {
	token := GetToken(ctx)
	if token == nil {
		return nil
	}
	return token.Claims.(*claims.JWTClaims)
}

func GetClaimsAsMap(ctx context.Context) map[string]interface{} {
	token := GetToken(ctx)
	if token == nil {
		return nil
	}
	config := GetConfig(ctx)
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

func WithAdminFlag(ctx context.Context, isAdmin bool) context.Context {
	return context.WithValue(ctx, adminFlagKey, isAdmin)
}

func IsAdmin(ctx context.Context) bool {
	obj := ctx.Value(adminFlagKey)
	if obj == nil {
		return false
	}
	return obj.(bool)
}
