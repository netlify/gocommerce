package api

import (
	"github.com/Sirupsen/logrus"
	"github.com/dgrijalva/jwt-go"
	"golang.org/x/net/context"

	"github.com/netlify/gocommerce/conf"
)

const (
	TokenKey  = "jwt"
	ConfigKey = "config"
	LoggerKey = "logger"
)

// RequestContext is a thin wrapper around the context to get useful features
type RequestContext struct {
	context.Context
}

func (rc *RequestContext) WithConfig(config *conf.Configuration) *RequestContext {
	rc.Context = context.WithValue(rc.Context, ConfigKey, config)
	return rc
}

func (rc RequestContext) Config() *conf.Configuration {
	obj := rc.Value(ConfigKey)
	if obj == nil {
		return nil
	}

	return obj.(*conf.Configuration)
}

func (rc *RequestContext) WithToken(token *jwt.Token) *RequestContext {
	rc.Context = context.WithValue(rc.Context, TokenKey, token)
	return rc
}

func (rc RequestContext) Token() *jwt.Token {
	obj := rc.Value(TokenKey)
	if obj == nil {
		return nil
	}

	return obj.(*jwt.Token)
}

func (rc RequestContext) Claims() *JWTClaims {
	token := rc.Token()
	if token == nil {
		return nil
	}
	return token.Claims.(*JWTClaims)
}

func (rc RequestContext) IsAdmin() bool {
	claims := rc.Claims()
	if claims == nil {
		return false
	}
	config := rc.Config()
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

func (rc *RequestContext) WithLogger(l *logrus.Entry) *RequestContext {
	rc.Context = context.WithValue(rc.Context, LoggerKey, l)
	return rc
}

func (rc RequestContext) Logger() *logrus.Entry {
	obj := rc.Value(LoggerKey)
	if obj == nil {
		return nil
	}
	return obj.(*logrus.Entry)
}
