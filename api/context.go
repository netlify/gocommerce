package api

import (
	"github.com/Sirupsen/logrus"
	"github.com/dgrijalva/jwt-go"
	"golang.org/x/net/context"

	"github.com/netlify/netlify-commerce/conf"
)

const (
	TokenKey     = "jwt"
	ConfigKey    = "config"
	LoggerKey    = "logger"
	RequestIDKey = "request_id"
)

func withLogger(ctx context.Context, l *logrus.Entry) context.Context {
	return context.WithValue(ctx, LoggerKey, l)
}

func withConfig(ctx context.Context, config *conf.Configuration) context.Context {
	return context.WithValue(ctx, ConfigKey, config)
}

func withToken(ctx context.Context, token *jwt.Token) context.Context {
	return context.WithValue(ctx, TokenKey, token)
}

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestIDKey, id)
}

func getRequestID(ctx context.Context) string {
	obj := ctx.Value(RequestIDKey)
	if obj == nil {
		return ""
	}

	return obj.(string)
}

func getConfig(ctx context.Context) *conf.Configuration {
	obj := ctx.Value(ConfigKey)
	if obj == nil {
		return nil
	}

	return obj.(*conf.Configuration)
}

func getToken(ctx context.Context) *jwt.Token {
	obj := ctx.Value(TokenKey)
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

func IsAdmin(ctx context.Context) bool {
	claims := getClaims(ctx)
	if claims == nil {
		return false
	}

	config := getConfig(ctx)
	if config == nil {
		return false
	}

	for _, g := range claims.Groups {
		if g == config.JWT.AdminGroupName {
			return true
		}
	}

	return false
}

func getLogger(ctx context.Context) *logrus.Entry {
	obj := ctx.Value(LoggerKey)
	if obj == nil {
		return logrus.NewEntry(logrus.StandardLogger())
	}
	return obj.(*logrus.Entry)
}
