package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/dgrijalva/jwt-go"
	"github.com/netlify/gocommerce/conf"
	"golang.org/x/net/context"
)

// HTTPError is an error with a message
type HTTPError struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
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

func userIDFromToken(token *jwt.Token) string {
	if token == nil {
		return ""
	}

	claims := token.Claims.(*JWTClaims)
	return claims.ID
}

// BadRequestError is simple Error Wrapper
func BadRequestError(w http.ResponseWriter, message string) {
	sendJSON(w, 400, &HTTPError{Code: 400, Message: message})
}

// UnprocessableEntity is simple Error Wrapper
func UnprocessableEntity(w http.ResponseWriter, message string) {
	sendJSON(w, 422, &HTTPError{Code: 422, Message: message})
}

// InternalServerError is simple Error Wrapper
func InternalServerError(w http.ResponseWriter, message string) {
	sendJSON(w, 500, &HTTPError{Code: 500, Message: message})
}

// NotFoundError is simple Error Wrapper
func NotFoundError(w http.ResponseWriter, message string) {
	sendJSON(w, 404, &HTTPError{Code: 404, Message: message})
}

// UnauthorizedError is simple Error Wrapper
func UnauthorizedError(w http.ResponseWriter, message string) {
	sendJSON(w, 401, &HTTPError{Code: 401, Message: message})
}

func calculateTotalPages(perPage, total uint64) uint64 {
	pages := total / perPage
	if total%perPage > 0 {
		return pages + 1
	}
	return pages
}

func addPaginationHeaders(w http.ResponseWriter, r *http.Request, page, perPage, total uint64) {
	totalPages := calculateTotalPages(perPage, total)
	url, _ := url.ParseRequestURI(r.URL.String())
	query := url.Query()
	header := ""
	if totalPages > page {
		query.Set("page", fmt.Sprintf("%v", page+1))
		url.RawQuery = query.Encode()
		header += "<" + url.String() + ">; rel=\"next\", "
	}
	query.Set("page", fmt.Sprintf("%v", totalPages))
	url.RawQuery = query.Encode()
	header += "<" + url.String() + ">; rel=\"last\""

	w.Header().Add("Link", header)
}
