package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/guregu/kami"
	"github.com/netlify/netlify-commerce/models"
	stripe "github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
)

// MaxConcurrentLookups controls the number of simultaneous HTTP Order lookups
const MaxConcurrentLookups = 10

// PaymentParams holds the parameters for creating a payment
type PaymentParams struct {
	Amount      uint64 `json:"amount"`
	Currency    string `json:"currency"`
	StripeToken string `json:"stripe_token"`
}

// PaymentList is the endpoint for listing transactions for an order
func (a *API) PaymentList(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	token := getToken(ctx)
	if token == nil {
		UnauthorizedError(w, "Listing payments requires authentication")
		return
	}

	orderID := kami.Param(ctx, "order_id")
	order := &models.Order{}
	if result := a.db.Preload("Transactions").First(order, "id = ?", orderID); result.Error != nil {
		if result.RecordNotFound() {
			NotFoundError(w, "Order not found")
		} else {
			InternalServerError(w, fmt.Sprintf("Error during database query: %v", result.Error))
		}
	}

	sendJSON(w, 200, order.Transactions)
}

// PaymentCreate is the endpoint for creating a payment for an order
func (a *API) PaymentCreate(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	params := &PaymentParams{Currency: "USD"}
	jsonDecoder := json.NewDecoder(r.Body)
	err := jsonDecoder.Decode(params)
	if err != nil {
		BadRequestError(w, fmt.Sprintf("Could not read params: %v", err))
		return
	}

	if params.StripeToken == "" {
		BadRequestError(w, "Payments requires a stripe_token")
		return
	}

	orderID := kami.Param(ctx, "order_id")
	tx := a.db.Begin()
	order := &models.Order{}

	if result := tx.Preload("LineItems").Preload("BillingAddress").First(order, "id = ?", orderID); result.Error != nil {
		tx.Rollback()
		if result.RecordNotFound() {
			NotFoundError(w, "No order with this ID found")
		} else {
			InternalServerError(w, fmt.Sprintf("Error during database query: %v", result.Error))
		}
		return
	}

	if order.PaymentState == models.PaidState {
		tx.Rollback()
		BadRequestError(w, "This order has already been paid")
		return
	}

	if order.Currency != params.Currency {
		tx.Rollback()
		BadRequestError(w, fmt.Sprintf("Currencies doesn't match - %v vs %v", order.Currency, params.Currency))
		return
	}

	token := getToken(ctx)
	if order.UserID == "" {
		if token != nil {
			claims := token.Claims.(*JWTClaims)
			order.UserID = claims.ID
			tx.Save(order)
		}
	} else {
		if token == nil {
			tx.Rollback()
			UnauthorizedError(w, "You must be logged in to pay for this order")
			return
		}
		claims := token.Claims.(*JWTClaims)
		if order.UserID != claims.ID {
			tx.Rollback()
			UnauthorizedError(w, "You must be logged in to pay for this order")
			return
		}
	}

	err = a.verifyAmount(ctx, order, params.Amount)
	if err != nil {
		tx.Rollback()
		InternalServerError(w, fmt.Sprintf("We failed to authorize the amount for this order: %v", err))
		return
	}

	ch, err := charge.New(&stripe.ChargeParams{
		Amount:   params.Amount,
		Source:   &stripe.SourceParams{Token: params.StripeToken},
		Currency: "USD",
	})
	tr := models.NewTransaction(order)
	if ch != nil {
		tr.ProcessorID = ch.ID
	}
	if err != nil {
		tr.FailureCode = "500"
		tr.FailureDescription = err.Error()
		tr.Status = "failed"
	} else {
		tr.Status = "pending"
	}
	tx.Create(tr)

	if err != nil {
		tx.Commit()
		InternalServerError(w, fmt.Sprintf("There was an error charging your card: %v", err))
		return
	}

	order.PaymentState = models.PaidState
	tx.Save(order)
	tx.Commit()

	go a.mailer.OrderConfirmationMail(tr)
	go a.mailer.OrderReceivedMail(tr)

	sendJSON(w, 200, tr)
}

func (a *API) verifyAmount(ctx context.Context, order *models.Order, amount uint64) error {
	config := getConfig(ctx)

	settings := &models.SiteSettings{}
	resp, err := a.httpClient.Get(config.SiteURL + "/netlify-commerce/settings.json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(settings); err != nil {
			return err
		}
	}

	order.CalculateTotal(settings)

	if order.Total != amount {
		return fmt.Errorf("Amount calculated for order didn't match amount to charge. %v vs %v", order.Total, amount)
	}

	return nil
}
