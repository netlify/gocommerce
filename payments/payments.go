package payments

import (
	"context"
	"net/http"

	"github.com/netlify/gocommerce/models"
	"github.com/sirupsen/logrus"
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
	NewCharger(ctx context.Context, r *http.Request, log logrus.FieldLogger) (Charger, error)
	NewRefunder(ctx context.Context, r *http.Request, log logrus.FieldLogger) (Refunder, error)
	NewPreauthorizer(ctx context.Context, r *http.Request, log logrus.FieldLogger) (Preauthorizer, error)
	NewConfirmer(ctx context.Context, r *http.Request, log logrus.FieldLogger) (Confirmer, error)
}

// Charger wraps the Charge method which creates new payments with the provider.
type Charger func(amount uint64, currency string, order *models.Order, invoiceNumber int64) (string, error)

// Refunder wraps the Refund method which refunds payments with the provider.
type Refunder func(transactionID string, amount uint64, currency string) (string, error)

// Preauthorizer wraps the Preauthorize method which pre-authorizes a payment
// with the provider.
type Preauthorizer func(amount uint64, currency string, description string) (*PreauthorizationResult, error)

// PreauthorizationResult contains the data returned from a Preauthorization.
type PreauthorizationResult struct {
	ID string `json:"id"`
}

// Confirmer wraps a confirm method used for checking two-step payments in a synchronous flow
type Confirmer func(paymentID string) error

// PaymentPendingError is returned when the payment provider requests additional action
// e.g. 2-step authorization through 3D secure
type PaymentPendingError struct {
	metadata map[string]interface{}
}

// NewPaymentPendingError creates an error for a pending action on a payment
func NewPaymentPendingError(metadata map[string]interface{}) error {
	return &PaymentPendingError{metadata}
}

func (p *PaymentPendingError) Error() string {
	return "The payment provider requested additional actions on the transaction."
}

// Metadata returns fields that should be passed to the client
// for use in additional actions
func (p *PaymentPendingError) Metadata() map[string]interface{} {
	return p.metadata
}

// PaymentConfirmFailError is returned when the confirmation request got a negative response
type PaymentConfirmFailError struct {
	message string
}

// NewPaymentConfirmFailError creates an error to use when a payment confirmation fails
func NewPaymentConfirmFailError(msg string) error {
	return &PaymentConfirmFailError{message: msg}
}

func (p *PaymentConfirmFailError) Error() string {
	return p.message
}
