package api

import (
	"github.com/Sirupsen/logrus"
	"github.com/dgrijalva/jwt-go"
	"golang.org/x/net/context"

	"github.com/netlify/gocommerce/conf"
)

const (
	TokenKey     = "jwt"
	ConfigKey    = "config"
	LoggerKey    = "logger"
	RequestIDKey = "request_id"
)

func WithLogger(ctx context.Context, l *logrus.Entry) context.Context {
	return context.WithValue(ctx, LoggerKey, l)
}

func WithConfig(ctx context.Context, config *conf.Configuration) context.Context {
	return context.WithValue(ctx, ConfigKey, config)
}

func WithToken(ctx context.Context, token *jwt.Token) context.Context {
	return context.WithValue(ctx, TokenKey, token)
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestIDKey, id)
}

func requestID(ctx context.Context) string {
	obj := ctx.Value(RequestIDKey)
	if obj == nil {
		return ""
	}

	return obj.(string)
}

func Config(ctx context.Context) *conf.Configuration {
	obj := ctx.Value(ConfigKey)
	if obj == nil {
		return nil
	}

	return obj.(*conf.Configuration)
}

func Token(ctx context.Context) *jwt.Token {
	obj := ctx.Value(TokenKey)
	if obj == nil {
		return nil
	}

	return obj.(*jwt.Token)
}

func Claims(ctx context.Context) *JWTClaims {
	token := Token(ctx)
	if token == nil {
		return nil
	}
	return token.Claims.(*JWTClaims)
}

func IsAdmin(ctx context.Context) bool {
	claims := Claims(ctx)
	if claims == nil {
		return false
	}
	config := Config(ctx)
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

func Logger(ctx context.Context) *logrus.Entry {
	obj := ctx.Value(LoggerKey)
	if obj == nil {
		return logrus.NewEntry(logrus.StandardLogger())
	}
	return obj.(*logrus.Entry)
}
