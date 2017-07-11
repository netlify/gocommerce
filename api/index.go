package api

import (
	"context"
	"net/http"
)

// Index endpoint
func (a *API) Index(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	sendJSON(w, http.StatusOK, map[string]string{
		"version":     a.version,
		"name":        "GoCommerce",
		"description": "GoCommerce is a flexible Ecommerce API for JAMStack sites",
	})
}
