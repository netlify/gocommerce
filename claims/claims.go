package claims

import (
	"strings"

	jwt "github.com/dgrijalva/jwt-go"
)

// JWTClaims represents the JWT claims information.
type JWTClaims struct {
	Email        string                 `json:"email"`
	AppMetaData  map[string]interface{} `json:"app_metadata"`
	UserMetaData map[string]interface{} `json:"user_metadata"`
	jwt.StandardClaims
}

// HasClaims is used to determine if a set of userClaims matches the requiredClaims
func HasClaims(userClaims map[string]interface{}, requiredClaims map[string]string) bool {
	if requiredClaims == nil {
		return true
	}
	if userClaims == nil {
		return false
	}

	for key, value := range requiredClaims {
		parts := strings.Split(key, ".")
		obj := userClaims
		for i, part := range parts {
			newObj, hasObj := obj[part]
			if !hasObj {
				return false
			}
			if i+1 == len(parts) {
				str, isString := newObj.(string)
				if !isString {
					return false
				}
				return str == value
			}
			obj, hasObj = newObj.(map[string]interface{})
			if !hasObj {
				return false
			}

		}
	}
	return false
}
