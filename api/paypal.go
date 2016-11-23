package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/guregu/kami"
	paypalsdk "github.com/logpacker/PayPal-Go-SDK"
)

// PaypalCreatePayment creates a new payment that can be authorized in the browser
func (a *API) PaypalCreatePayment(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	amount := paypalsdk.Amount{
		Total:    r.FormValue("amount"),
		Currency: r.FormValue("currency"),
	}
	redirectURI := a.config.SiteURL + "/netlify-commerce/paypal"
	cancelURI := a.config.SiteURL + "/netlify-commerce/paypal/cancel"
	paymentResult, err := a.paypal.CreateDirectPaypalPayment(amount, redirectURI, cancelURI, r.FormValue("description"))
	if err != nil {
		internalServerError(w, fmt.Sprintf("Error creating paypal payment: %v", err))
		return
	}

	sendJSON(w, 200, paymentResult)
}

// PaypalGetPayment retrieves information on an authorized paypal payment, including
// the shipping address
func (a *API) PaypalGetPayment(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	payment, err := a.paypal.GetPayment(kami.Param(ctx, "payment_id"))
	if err != nil {
		internalServerError(w, fmt.Sprintf("Error fetching paypal payment: %v", err))
		return
	}

	sendJSON(w, 200, payment)
}
