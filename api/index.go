package api

import (
	"net/http"

	"golang.org/x/net/context"
)

const description = `{
  "name": "GoCommerce",
  "description": "GoCommerce is a flexible Ecommerce API for JAM sites"
}`

// Index endpoint
func (a *API) Index(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(description))
}
