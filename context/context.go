package context

import (
	"fmt"

	"context"

	"github.com/dgrijalva/jwt-go"

	"github.com/netlify/gocommerce/assetstores"
	"github.com/netlify/gocommerce/claims"
	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/coupons"
	"github.com/netlify/gocommerce/mailer"
	"github.com/netlify/gocommerce/models"
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
	requestIDKey       = contextKey("request_id")
	adminFlagKey       = contextKey("is_admin")
	mailerKey          = contextKey("mailer")
	assetStoreKey      = contextKey("asset_store")
	paymentProviderKey = contextKey("payment-provider")
	userIDKey          = contextKey("user_id")
	userKey            = contextKey("user")
	orderIDKey         = contextKey("order_id")
	instanceIDKey      = contextKey("instance_id")
	instanceKey        = contextKey("instance")
)

// WithConfig adds the tenant configuration to the context.
func WithConfig(ctx context.Context, config *conf.Configuration) context.Context {
	return context.WithValue(ctx, configKey, config)
}

// GetConfig reads the tenant configuration from the context.
func GetConfig(ctx context.Context) *conf.Configuration {
	obj := ctx.Value(configKey)
	if obj == nil {
		return nil
	}

	return obj.(*conf.Configuration)
}

// WithCoupons adds the coupon cache to the context based on the site URL.
func WithCoupons(ctx context.Context, config *conf.Configuration) (context.Context, error) {
	cache, err := coupons.NewCouponCacheFromURL(config)
	if err != nil {
		return nil, err
	}
	return context.WithValue(ctx, couponsKey, cache), nil
}

// GetCoupons reads the coupon cache from the context.
func GetCoupons(ctx context.Context) coupons.Cache {
	obj := ctx.Value(couponsKey)
	if obj == nil {
		return nil
	}

	return obj.(coupons.Cache)
}

// WithToken adds the JWT token to the context.
func WithToken(ctx context.Context, token *jwt.Token) context.Context {
	return context.WithValue(ctx, tokenKey, token)
}

// GetToken reads the JWT token from the context.
func GetToken(ctx context.Context) *jwt.Token {
	obj := ctx.Value(tokenKey)
	if obj == nil {
		return nil
	}

	return obj.(*jwt.Token)
}

// WithRequestID adds the provided request ID to the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// GetRequestID reads the request ID from the context.
func GetRequestID(ctx context.Context) string {
	obj := ctx.Value(requestIDKey)
	if obj == nil {
		return ""
	}

	return obj.(string)
}

// WithMailer adds the mailer to the context.
func WithMailer(ctx context.Context, mailer mailer.Mailer) context.Context {
	return context.WithValue(ctx, mailerKey, mailer)
}

// GetMailer reads the mailer from the context.
func GetMailer(ctx context.Context) mailer.Mailer {
	obj := ctx.Value(mailerKey)
	if obj == nil {
		return nil
	}
	return obj.(mailer.Mailer)
}

// WithAssetStore adds the asset store to the context.
func WithAssetStore(ctx context.Context, store assetstores.Store) context.Context {
	return context.WithValue(ctx, assetStoreKey, store)
}

// GetAssetStore reads the asset store from the context.
func GetAssetStore(ctx context.Context) assetstores.Store {
	obj := ctx.Value(assetStoreKey)
	if obj == nil {
		return nil
	}
	return obj.(assetstores.Store)
}

// WithPaymentProviders adds the payment providers to the context.
func WithPaymentProviders(ctx context.Context, provs map[string]payments.Provider) context.Context {
	return context.WithValue(ctx, paymentProviderKey, provs)
}

// GetPaymentProviders reads the payment providers from the context
func GetPaymentProviders(ctx context.Context) map[string]payments.Provider {
	provs, _ := ctx.Value(paymentProviderKey).(map[string]payments.Provider)
	return provs
}

// GetClaims reads the claims contained within the JWT token stored in the context.
func GetClaims(ctx context.Context) *claims.JWTClaims {
	token := GetToken(ctx)
	if token == nil {
		return nil
	}
	return token.Claims.(*claims.JWTClaims)
}

// GetClaimsAsMap reads the claims contained with the JWT token stored in the
// context, as a map.
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
	_, err := jwt.ParseWithClaims(token.Raw, &claims, func(token *jwt.Token) (interface{}, error) {
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

// WithAdminFlag adds a flag indicating admin status to the context.
func WithAdminFlag(ctx context.Context, isAdmin bool) context.Context {
	return context.WithValue(ctx, adminFlagKey, isAdmin)
}

// IsAdmin reads the admin flag from the context.
func IsAdmin(ctx context.Context) bool {
	obj := ctx.Value(adminFlagKey)
	if obj == nil {
		return false
	}
	return obj.(bool)
}

// GetUserID reads the user ID from the context.
func GetUserID(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey).(string)
	return id
}

// WithUserID adds the user ID to the context.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// GetUser reads the user from the context.
func GetUser(ctx context.Context) *models.User {
	u := ctx.Value(userKey)
	if u == nil {
		return nil
	}
	return u.(*models.User)
}

// WithUser adds the user to the context.
func WithUser(ctx context.Context, user *models.User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

// GetOrderID reads the order ID from the context.
func GetOrderID(ctx context.Context) string {
	id, _ := ctx.Value(orderIDKey).(string)
	return id
}

// WithOrderID adds the order ID to the context.
func WithOrderID(ctx context.Context, orderID string) context.Context {
	return context.WithValue(ctx, orderIDKey, orderID)
}

// WithInstanceID adds the instance id to the context.
func WithInstanceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, instanceIDKey, id)
}

// GetInstanceID reads the instance id from the context.
func GetInstanceID(ctx context.Context) string {
	obj := ctx.Value(instanceIDKey)
	if obj == nil {
		return ""
	}
	return obj.(string)
}

// WithInstance adds the instance id to the context.
func WithInstance(ctx context.Context, i *models.Instance) context.Context {
	return context.WithValue(ctx, instanceKey, i)
}

// GetInstance reads the instance id from the context.
func GetInstance(ctx context.Context) *models.Instance {
	obj := ctx.Value(instanceKey)
	if obj == nil {
		return nil
	}
	return obj.(*models.Instance)
}
