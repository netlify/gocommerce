package payments

import (
	"context"
	"net/http"
)

const (
	StripeProvider = "stripe"
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
type Charger interface {
	Charge(amount uint64, currency string) (string, error)
}

// Refunder wraps the Refund method which refunds payments with the provider.
type Refunder interface {
	Refund(transactionID string, amount uint64) (string, error)
}

// Preauthorizer wraps the Preauthorize method which pre-authorizes a payment
// with the provider.
type Preauthorizer interface {
	Preauthorize(amount uint64, currency string, description string) (*PreauthorizationResult, error)
}

// PreauthorizationResult contains the data returned from a Preauthorization.
type PreauthorizationResult struct {
	ID string `json:"id"`
}
