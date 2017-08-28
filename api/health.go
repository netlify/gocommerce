package api

import (
	"net/http"
)

// HealthCheck endpoint
func (a *API) HealthCheck(w http.ResponseWriter, r *http.Request) error {
	return sendJSON(w, http.StatusOK, map[string]string{
		"version":     a.version,
		"name":        "GoCommerce",
		"description": "GoCommerce is a flexible Ecommerce API for JAMStack sites",
	})
}
