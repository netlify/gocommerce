package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/guregu/kami"
	paypalsdk "github.com/logpacker/PayPal-Go-SDK"
)

type Experience struct {
	profile *paypalsdk.WebProfile
	mutex   sync.Mutex
}

var paypalExperience Experience

// PaypalCreatePayment creates a new payment that can be authorized in the browser
func (a *API) PaypalCreatePayment(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	profile, err := a.getExperience()
	if err != nil {
		internalServerError(w, fmt.Sprintf("Error creating paypal experience: %v", err))
		return
	}

	amount := &paypalsdk.Amount{
		Total:    r.FormValue("amount"),
		Currency: r.FormValue("currency"),
	}
	a.log.Infof("Creating paypal payment with profile %v: %v", profile, amount)
	redirectURI := a.config.SiteURL + "/netlify-commerce/paypal"
	cancelURI := a.config.SiteURL + "/netlify-commerce/paypal/cancel"
	paymentResult, err := a.paypal.CreatePayment(paypalsdk.Payment{
		Intent: "sale",
		Payer: &paypalsdk.Payer{
			PaymentMethod: "paypal",
		},
		ExperienceProfileID: profile.ID,
		Transactions: []paypalsdk.Transaction{paypalsdk.Transaction{
			Amount:      amount,
			Description: r.FormValue("description"),
		}},
		RedirectURLs: &paypalsdk.RedirectURLs{
			ReturnURL: redirectURI,
			CancelURL: cancelURI,
		},
	})

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

func (a *API) getExperience() (*paypalsdk.WebProfile, error) {
	if paypalExperience.profile != nil {
		return paypalExperience.profile, nil
	}

	experiences, err := a.paypal.GetWebProfiles()
	if err != nil {
		a.log.Errorf("Error getting web profiles: %v", err)
		return nil, err
	}

	for _, profile := range experiences {
		if profile.Name == "netlify-commerce" {
			paypalExperience.mutex.Lock()
			paypalExperience.profile = &profile
			paypalExperience.mutex.Unlock()
			return paypalExperience.profile, nil
		}
	}

	profile, err := a.paypal.CreateWebProfile(paypalsdk.WebProfile{
		Name: "netlify-commerce",
		InputFields: paypalsdk.InputFields{
			NoShipping: 1,
		},
	})

	if err != nil {
		a.log.Errorf("Error creating web profile: %v", err)
		return nil, err
	}

	paypalExperience.mutex.Lock()
	paypalExperience.profile = profile
	paypalExperience.mutex.Unlock()

	return profile, nil
}
