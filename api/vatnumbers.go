package api

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/mattes/vat"
)

// VatNumberLookup looks up information on a VAT number
func (a *API) VatNumberLookup(w http.ResponseWriter, r *http.Request) error {
	number := chi.URLParam(r, "vat_number")

	response, err := vat.CheckVAT(number)
	if err != nil {
		return internalServerError("Failed to lookup VAT Number").WithInternalError(err)
	}

	return sendJSON(w, http.StatusOK, map[string]interface{}{
		"country": response.CountryCode,
		"valid":   response.Valid,
		"company": response.Name,
		"address": response.Address,
	})
}
