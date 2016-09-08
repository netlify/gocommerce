package api

import (
	"fmt"
	"net/http"

	"golang.org/x/net/context"

	"github.com/mattes/vat"
	"github.com/rybit/kami"
)

func (a *API) VatnumberLookup(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	number := kami.Param(ctx, "number")

	response, err := vat.CheckVAT(number)
	if err != nil {
		InternalServerError(w, fmt.Sprintf("Failed to lookup VAT Number: %v", err))
		return
	}

	sendJSON(w, 200, map[string]interface{}{
		"country": response.CountryCode,
		"valid":   response.Valid,
		"company": response.Name,
		"address": response.Address,
	})
}
