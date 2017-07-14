package payments

import (
	"context"
	"net/http"
)

const (
	// StripeProvider is the string identifier for the Stripe payment provider.
	StripeProvider = "stripe"
	// PayPalProvider is the string identifier for the PayPal payment provider.
	PayPalProvider = "paypal"
)

// Provider represents a payment provider that can optionally charge, refund,
// preauthorize payments.
type Provider interface {
	Name() string
	NewCharger(ctx context.Context, r *http.Request) (Charger, error)
	NewRefunder(ctx context.Context, r *http.Request) (Refunder, error)
	NewPreauthorizer(ctx context.Context, r *http.Request) (Preauthorizer, error)
}

// Charger wraps the Charge method which creates new payments with the provider.
type Charger func(amount uint64, currency string) (string, error)

// Refunder wraps the Refund method which refunds payments with the provider.
type Refunder func(transactionID string, amount uint64, currency string) (string, error)

// Preauthorizer wraps the Preauthorize method which pre-authorizes a payment
// with the provider.
type Preauthorizer func(amount uint64, currency string, description string) (*PreauthorizationResult, error)

// PreauthorizationResult contains the data returned from a Preauthorization.
type PreauthorizationResult struct {
	ID string `json:"id"`
}
