package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/netlify/gocommerce/claims"
	gcontext "github.com/netlify/gocommerce/context"
	"github.com/netlify/gocommerce/models"
)

func withTokenCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := getLogEntry(r)
		config := gcontext.GetConfig(ctx)
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			log.Info("Making unauthenticated request")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		matches := bearerRegexp.FindStringSubmatch(authHeader)
		if len(matches) != 2 {
			log.Infof("Invalid auth header format: %s", authHeader)
			unauthorizedError(w, "Bad authentication header")
			return
		}

		token, err := jwt.ParseWithClaims(matches[1], &claims.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
			if token.Method.Alg() != jwt.SigningMethodHS256.Name {
				return nil, fmt.Errorf("Unexpected signing method: %v", token.Method.Alg())
			}
			return []byte(config.JWT.Secret), nil
		})
		if err != nil {
			log.WithError(err).Infof("Invalid token")
			unauthorizedError(w, "Invalid token")
			return
		}

		claims := token.Claims.(*claims.JWTClaims)
		// I'm pretty sure the library is already validating the expiration
		// if claims.StandardClaims.ExpiresAt < time.Now().Unix() {
		// 	msg := fmt.Sprintf("Token expired at %v", time.Unix(claims.StandardClaims.ExpiresAt, 0))
		// 	log.Info(msg)
		// 	unauthorizedError(w, msg)
		// 	return
		// }

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

		log.WithFields(logrus.Fields{
			"claims_id":    claims.ID,
			"claims_email": claims.Email,
			"roles":        roles,
			"is_admin":     isAdmin,
		}).Info("successfully parsed claims")

		ctx = gcontext.WithAdminFlag(ctx, isAdmin)
		ctx = gcontext.WithToken(ctx, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func authRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := getLogEntry(r)
		claims := gcontext.GetClaims(ctx)
		if claims == nil {
			err := unauthorizedError(w, "No claims provided")
			log.WithError(err).Warn("Illegal access attempt")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func adminRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := getLogEntry(r)
		claims := gcontext.GetClaims(ctx)
		isAdmin := gcontext.IsAdmin(ctx)

		if claims == nil || !isAdmin {
			err := unauthorizedError(w, "Admin permissions required")
			log.WithError(err).Warn("Illegal access attempt")
			return
		}

		logEntrySetField(r, "admin_id", claims.ID)
		next.ServeHTTP(w, r)
	})
}

func ensureUserAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := getLogEntry(r)
		ctx := r.Context()
		userID := getUserID(ctx)

		// ensure userID matches authenticated user OR is admin
		claims := gcontext.GetClaims(ctx)
		isAdmin := gcontext.IsAdmin(ctx)
		if claims.ID != userID && !isAdmin {
			err := unauthorizedError(w, "Can't access a different user unless you're an admin")
			log.WithError(err).Warn("Illegal access attempt")
			return
		}
		if isAdmin {
			logEntrySetField(r, "admin_id", claims.ID)
		}

		next.ServeHTTP(w, r)
	})
}

func hasOrderAccess(ctx context.Context, order *models.Order) bool {
	claims := gcontext.GetClaims(ctx)

	if order.UserID != "" {
		if claims == nil || (order.UserID != claims.ID && !gcontext.IsAdmin(ctx)) {
			return false
		}
	}
	return true
}
