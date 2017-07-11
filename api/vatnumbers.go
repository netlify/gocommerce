package api

import (
	"context"
	"net/http"

	"github.com/guregu/kami"
	"github.com/mattes/vat"
)

func (a *API) VatnumberLookup(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	number := kami.Param(ctx, "number")

	response, err := vat.CheckVAT(number)
	if err != nil {
		internalServerError(w, "Failed to lookup VAT Number: %v", err)
		return
	}

	sendJSON(w, 200, map[string]interface{}{
		"country": response.CountryCode,
		"valid":   response.Valid,
		"company": response.Name,
		"address": response.Address,
	})
}
