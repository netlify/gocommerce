package api

import (
	"fmt"
	"net/http"

	"github.com/netlify/gocommerce/calculator"
	gcontext "github.com/netlify/gocommerce/context"
)

func (a *API) ViewSettings(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	config := gcontext.GetConfig(ctx)

	settings, err := a.loadSettings(ctx)
	if err != nil {
		return fmt.Errorf("Error loading site settings: %v", err)
	}

	pms := &calculator.PaymentMethods{}
	if config.Payment.Stripe.Enabled {
		pms.Stripe.Enabled = true
		pms.Stripe.PublicKey = config.Payment.Stripe.PublicKey
	}
	if config.Payment.PayPal.Enabled {
		pms.PayPal.Enabled = true
		pms.PayPal.ClientID = config.Payment.PayPal.ClientID
		pms.PayPal.Environment = config.Payment.PayPal.Env
	}
	settings.PaymentMethods = pms

	sendJSON(w, 200, settings)
	return nil
}
