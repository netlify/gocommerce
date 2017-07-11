package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/netlify/gocommerce/claims"
	gcontext "github.com/netlify/gocommerce/context"
)

func (a *API) withToken(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	log := gcontext.GetLogger(ctx)
	config := gcontext.GetConfig(ctx)
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		log.Info("Making unauthenticated request")
		return ctx
	}

	matches := bearerRegexp.FindStringSubmatch(authHeader)
	if len(matches) != 2 {
		log.Info("Invalid auth header format: " + authHeader)
		unauthorizedError(w, "Bad authentication header")
		return nil
	}

	token, err := jwt.ParseWithClaims(matches[1], &claims.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Header["alg"] != jwt.SigningMethodHS256.Name {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.JWT.Secret), nil
	})
	if err != nil {
		log.Infof("Invalid token: %v", err)
		unauthorizedError(w, "Invalid token")
		return nil
	}

	claims := token.Claims.(*claims.JWTClaims)
	if claims.StandardClaims.ExpiresAt < time.Now().Unix() {
		msg := fmt.Sprintf("Token expired at %v", time.Unix(claims.StandardClaims.ExpiresAt, 0))
		log.Info(msg)
		unauthorizedError(w, msg)
		return nil
	}

	isAdmin := false
	roles, ok := claims.AppMetaData["roles"]
	if ok {
		roleStrings, _ := roles.([]interface{})
		for _, data := range roleStrings {
			role, _ := data.(string)
			if role == config.JWT.AdminGroupName {
				isAdmin = true
				break
			}
		}
	}

	log = log.WithFields(logrus.Fields{
		"claims_id":    claims.ID,
		"claims_email": claims.Email,
		"roles":        roles,
		"is_admin":     isAdmin,
	})

	log.Info("successfully parsed claims")
	ctx = gcontext.WithAdminFlag(ctx, isAdmin)
	ctx = gcontext.WithLogger(ctx, log)

	return gcontext.WithToken(ctx, token)
}
