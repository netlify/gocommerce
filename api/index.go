package api

import (
	"context"
	"net/http"
)

const description = `{
  "name": "Netlify Commerce",
  "description": "Netlify Commerce is a flexible Ecommerce API for JAMStack sites"
}`

// Index endpoint
func (a *API) Index(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(description))
}
