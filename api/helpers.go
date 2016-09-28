package api

import (
	"encoding/json"
	"net/http"

	jwt "github.com/dgrijalva/jwt-go"
)

func sendJSON(w http.ResponseWriter, status int, obj interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.Encode(obj)
}

func userIDFromToken(token *jwt.Token) string {
	if token == nil {
		return ""
	}

	claims := token.Claims.(*JWTClaims)
	return claims.ID
}
