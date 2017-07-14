package api

import (
	"context"
	"net/http"

	"github.com/guregu/kami"
	"github.com/mattes/vat"
)

// VatNumberLookup looks up information on a VAT number
func (a *API) VatNumberLookup(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	number := kami.Param(ctx, "number")

	response, err := vat.CheckVAT(number)
	if err != nil {
		internalServerError(w, "Failed to lookup VAT Number: %v", err)
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"country": response.CountryCode,
		"valid":   response.Valid,
		"company": response.Name,
		"address": response.Address,
	})
}
