package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/dgrijalva/jwt-go"
	"github.com/netlify/gocommerce/conf"
	"golang.org/x/net/context"
)

// HTTPError is an error with a message
type HTTPError struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("%d: %s", e.Code, e.Message)
}

func httpError(code int, fmtString string, args ...interface{}) *HTTPError {
	return &HTTPError{
		Code:    code,
		Message: fmt.Sprintf(fmtString, args...),
	}
}

func sendJSON(w http.ResponseWriter, status int, obj interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.Encode(obj)
}

func getToken(ctx context.Context) *jwt.Token {
	obj := ctx.Value("jwt")
	if obj == nil {
		return nil
	}
	return obj.(*jwt.Token)
}

func getConfig(ctx context.Context) *conf.Configuration {
	obj := ctx.Value("config")
	if obj == nil {
		return nil
	}
	return obj.(*conf.Configuration)
}

func isAdmin(ctx context.Context, claims *JWTClaims) bool {
	if claims == nil {
		return false
	}

	config := getConfig(ctx)
	if config == nil {
		return false
	}

	for _, v := range claims.Groups {
		if v == config.JWT.AdminGroupName {
			return true
		}
	}

	return false
}

func userIDFromToken(token *jwt.Token) string {
	if token == nil {
		return ""
	}

	claims := token.Claims.(*JWTClaims)
	return claims.ID
}
