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

func withTokenCtx(w http.ResponseWriter, r *http.Request) (context.Context, error) {
	ctx := r.Context()
	log := getLogEntry(r)
	config := gcontext.GetConfig(ctx)
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		log.Info("Making unauthenticated request")
		return ctx, nil
	}

	matches := bearerRegexp.FindStringSubmatch(authHeader)
	if len(matches) != 2 {
		return nil, unauthorizedError("Bad authentication header").WithInternalMessage("Invalid auth header format: %s", authHeader)
	}

	token, err := jwt.ParseWithClaims(matches[1], &claims.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Name {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Method.Alg())
		}
		return []byte(config.JWT.Secret), nil
	})
	if err != nil {
		return nil, unauthorizedError("Invalid token").WithInternalError(err)
	}

	claims := token.Claims.(*claims.JWTClaims)
	// I'm pretty sure the library is already validating the expiration
	// if claims.StandardClaims.ExpiresAt < time.Now().Unix() {
	// 	return unauthorizedError("Token expired at %v", time.Unix(claims.StandardClaims.ExpiresAt, 0))
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
	return ctx, nil
}

func authRequired(w http.ResponseWriter, r *http.Request) (context.Context, error) {
	ctx := r.Context()
	claims := gcontext.GetClaims(ctx)
	if claims == nil {
		return nil, unauthorizedError("No claims provided")
	}

	return ctx, nil
}

func adminRequired(w http.ResponseWriter, r *http.Request) (context.Context, error) {
	ctx := r.Context()
	claims := gcontext.GetClaims(ctx)
	isAdmin := gcontext.IsAdmin(ctx)

	if claims == nil || !isAdmin {
		return nil, unauthorizedError("Admin permissions required")
	}

	logEntrySetField(r, "admin_id", claims.ID)
	return ctx, nil
}

func ensureUserAccess(w http.ResponseWriter, r *http.Request) (context.Context, error) {
	ctx := r.Context()
	userID := getUserID(ctx)

	// ensure userID matches authenticated user OR is admin
	claims := gcontext.GetClaims(ctx)
	isAdmin := gcontext.IsAdmin(ctx)
	if claims.ID != userID && !isAdmin {
		return nil, unauthorizedError("Can't access a different user unless you're an admin")
	}
	if isAdmin {
		logEntrySetField(r, "admin_id", claims.ID)
	}

	return ctx, nil
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
