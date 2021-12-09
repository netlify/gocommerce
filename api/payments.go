package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"strings"

	"github.com/go-chi/chi"

	"github.com/jinzhu/gorm"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"

	"mime"

	"github.com/netlify/gocommerce/claims"
	"github.com/netlify/gocommerce/conf"
	gcontext "github.com/netlify/gocommerce/context"
	"github.com/netlify/gocommerce/models"
	"github.com/netlify/gocommerce/payments"
	"github.com/netlify/gocommerce/payments/paypal"
	"github.com/netlify/gocommerce/payments/stripe"
)

// PaymentParams holds the parameters for creating a payment
type PaymentParams struct {
	Amount       uint64 `json:"amount"`
	Currency     string `json:"currency"`
	ProviderType string `json:"provider"`
	Description  string `json:"description"`
}

// PaymentListForUser is the endpoint for listing transactions for a user.
// The ID in the claim and the ID in the path must match (or have admin override)
func (a *API) PaymentListForUser(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := getLogEntry(r)
	userID := gcontext.GetUserID(ctx)
	user := gcontext.GetUser(ctx)
	if user == nil {
		return notFoundError("Couldn't find a record for " + userID)
	}

	trans, httpErr := queryForTransactions(a.DB(r), log, "user_id = ?", userID)
	if httpErr != nil {
		return httpErr
	}
	return sendJSON(w, http.StatusOK, trans)
}

// PaymentListForOrder is the endpoint for listing transactions for an order. You must be the owner
// of the order (user_id) or an admin. Listing the payments for an anon order.
func (a *API) PaymentListForOrder(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := getLogEntry(r)
	orderID := gcontext.GetOrderID(ctx)
	claims := gcontext.GetClaims(ctx)

	order, httpErr := queryForOrder(a.DB(r), orderID, log)
	if httpErr != nil {
		return httpErr
	}

	if !hasOrderAccess(ctx, order) {
		return unauthorizedError("You don't have access to this order").WithInternalMessage("Attempt to access order as %s, but order.UserID is %s", claims.Subject, order.UserID)
	}

	// additional check for anonymous orders: only allow admins
	isAdmin := gcontext.IsAdmin(ctx)
	if order.UserID == "" && !isAdmin {
		// anon order ~ only accessible by an admin
		return unauthorizedError("Anonymous orders must be accessed by admins")
	}

	log.Debugf("Returning %d transactions", len(order.Transactions))
	return sendJSON(w, http.StatusOK, order.Transactions)
}

func paymentComplete(r *http.Request, tx *gorm.DB, tr *models.Transaction, order *models.Order) {
	ctx := r.Context()
	log := getLogEntry(r)
	config := gcontext.GetConfig(ctx)

	tr.Status = models.PaidState
	if tx.NewRecord(tr) {
		tx.Create(tr)
	} else {
		tx.Save(tr)
	}
	order.PaymentState = models.PaidState
	tx.Save(order)

	if config.Webhooks.Payment != "" {
		hook, err := models.NewHook("payment", config.SiteURL, config.Webhooks.Payment, order.UserID, config.Webhooks.Secret, order)
		if err != nil {
			log.WithError(err).Error("Failed to process webhook")
		}
		tx.Save(hook)
	}
}

func sendOrderConfirmation(ctx context.Context, log logrus.FieldLogger, tr *models.Transaction) {
	mailer := gcontext.GetMailer(ctx)

	err1 := mailer.OrderConfirmationMail(tr)
	err2 := mailer.OrderReceivedMail(tr)

	if err1 != nil || err2 != nil {
		log.Errorf("Error sending order confirmation mails: %v %v", err1, err2)
	}
}

// PaymentCreate is the endpoint for creating a payment for an order
func (a *API) PaymentCreate(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := getLogEntry(r)

	params := PaymentParams{Currency: "USD"}
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		return badRequestError("Could not read params: %v", err)
	}
	if params.ProviderType == "" {
		return badRequestError("Creating a payment requires specifying a 'provider'")
	}

	provider := gcontext.GetPaymentProviders(ctx)[strings.ToLower(params.ProviderType)]
	if provider == nil {
		return badRequestError("Payment provider '%s' not configured", params.ProviderType)
	}
	charge, err := provider.NewCharger(ctx, r, log.WithField("component", "payment_provider"))
	if err != nil {
		return badRequestError("Error creating payment provider: %v", err)
	}

	orderID := gcontext.GetOrderID(ctx)
	tx := a.DB(r).Begin()
	order := &models.Order{}
	loader := tx.
		Preload("LineItems").
		Preload("Downloads").
		Preload("BillingAddress").
		Preload("ShippingAddress")
	if result := loader.First(order, "id = ?", orderID); result.Error != nil {
		tx.Rollback()
		if result.RecordNotFound() {
			return notFoundError("No order with this ID found")
		}
		return internalServerError("Error during database query").WithInternalError(result.Error)
	}

	if order.PaymentState == models.PaidState {
		tx.Rollback()
		return badRequestError("This order has already been paid")
	}

	if order.Currency != params.Currency {
		tx.Rollback()
		return badRequestError("Currencies doesn't match - %v vs %v", order.Currency, params.Currency)
	}

	token := gcontext.GetToken(ctx)
	if order.UserID == "" {
		if token != nil {
			claims := token.Claims.(*claims.JWTClaims)
			order.UserID = claims.Subject
			tx.Save(order)
		}
	} else {
		if token == nil {
			tx.Rollback()
			return unauthorizedError("You must be logged in to pay for this order")
		}
		claims := token.Claims.(*claims.JWTClaims)
		if order.UserID != claims.Subject {
			tx.Rollback()
			return unauthorizedError("You must be logged in to pay for this order")
		}
	}

	err = a.verifyAmount(ctx, order, params.Amount)
	if err != nil {
		tx.Rollback()
		return internalServerError("We failed to authorize the amount for this order: %v", err)
	}

	invoiceNumber := order.InvoiceNumber
	if invoiceNumber == 0 {
		var err error
		invoiceNumber, err = models.NextInvoiceNumber(tx, order.InstanceID)
		if err != nil {
			tx.Rollback()
			return internalServerError("We failed to generate a valid invoice ID, please try again later: %v", err)
		}
		order.InvoiceNumber = invoiceNumber
	}

	tr := models.NewTransaction(order)
	processorID, err := charge(params.Amount, params.Currency, order, invoiceNumber)
	tr.ProcessorID = processorID
	tr.InvoiceNumber = invoiceNumber
	order.PaymentProcessor = provider.Name()

	if err != nil {
		if pendingErr, ok := err.(*payments.PaymentPendingError); ok {
			tr.Status = models.PendingState
			tr.ProviderMetadata = pendingErr.Metadata()
			tx.Create(tr)
			tx.Save(order)
			tx.Commit()
			return sendJSON(w, 200, tr)
		}

		tr.FailureCode = strconv.FormatInt(http.StatusInternalServerError, 10)
		tr.FailureDescription = err.Error()
		tr.Status = models.FailedState
		tx.Create(tr)
		tx.Commit()
		return internalServerError("There was an error charging your card: %v", err).WithInternalError(err)
	}

	paymentComplete(r, tx, tr, order)
	if err := tx.Commit().Error; err != nil {
		return internalServerError("Saving payment failed").WithInternalError(err)
	}

	go sendOrderConfirmation(ctx, log, tr)

	return sendJSON(w, http.StatusOK, tr)
}

// PaymentConfirm allows client to confirm if a pending transaction has been completed. Updates transaction and order
func (a *API) PaymentConfirm(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := getLogEntry(r)
	db := a.DB(r)

	payID := chi.URLParam(r, "payment_id")
	trans, httpErr := getTransaction(db, payID)
	if httpErr != nil {
		return httpErr
	}

	if trans.UserID != "" {
		token := gcontext.GetToken(ctx)
		if token == nil {
			return unauthorizedError("You must be logged in to confirm this payment")
		}
		claims := token.Claims.(*claims.JWTClaims)
		if trans.UserID != claims.Subject {
			return unauthorizedError("You must be logged in to confirm this payment")
		}
	}

	if trans.Status == models.PaidState {
		return sendJSON(w, http.StatusOK, trans)
	}

	order := &models.Order{}
	if rsp := db.Find(order, "id = ?", trans.OrderID); rsp.Error != nil {
		if rsp.RecordNotFound() {
			return notFoundError("Order not found")
		}
		return internalServerError("Error while querying for order").WithInternalError(rsp.Error)
	}
	if order.PaymentProcessor == "" {
		return badRequestError("Order does not specify a payment provider")
	}

	trans.Order = order

	provider := gcontext.GetPaymentProviders(ctx)[order.PaymentProcessor]
	if provider == nil {
		return badRequestError("Payment provider '%s' not configured", order.PaymentProcessor)
	}
	confirm, err := provider.NewConfirmer(ctx, r, log.WithField("component", "payment_provider"))
	if err != nil {
		return badRequestError("Error creating payment provider: %v", err)
	}

	if err := confirm(trans.ProcessorID); err != nil {
		if confirmFail, ok := err.(*payments.PaymentConfirmFailError); ok {
			return badRequestError("Error confirming payment: %s", confirmFail.Error())
		}
		return internalServerError("Error on provider while trying to confirm: %v. Try again later.", err)
	}

	tx := db.Begin()

	if trans.InvoiceNumber == 0 {
		invoiceNumber, err := models.NextInvoiceNumber(tx, order.InstanceID)
		if err != nil {
			tx.Rollback()
			return internalServerError("We failed to generate a valid invoice ID, please try again later: %v", err)
		}
		trans.InvoiceNumber = invoiceNumber
	}

	paymentComplete(r, tx, trans, order)
	if err := tx.Commit().Error; err != nil {
		return internalServerError("Saving payment failed").WithInternalError(err)
	}

	go sendOrderConfirmation(ctx, log, trans)

	return sendJSON(w, http.StatusOK, trans)
}

// PaymentList will list all the payments that meet the criteria. It is only available to admins.
func (a *API) PaymentList(w http.ResponseWriter, r *http.Request) error {
	log := getLogEntry(r)
	instanceID := gcontext.GetInstanceID(r.Context())
	query := a.DB(r).Where("instance_id = ?", instanceID)

	query, err := parsePaymentQueryParams(query, r.URL.Query())
	if err != nil {
		return badRequestError("Malformed request: %v", err)
	}

	trans, httpErr := queryForTransactions(query, log, "", "")
	if httpErr != nil {
		return httpErr
	}
	return sendJSON(w, http.StatusOK, trans)
}

// PaymentView returns information about a single payment. It is only available to admins.
func (a *API) PaymentView(w http.ResponseWriter, r *http.Request) error {
	payID := chi.URLParam(r, "payment_id")
	trans, httpErr := getTransaction(a.DB(r), payID)
	if httpErr != nil {
		return httpErr
	}
	return sendJSON(w, http.StatusOK, trans)
}

// PaymentRefund refunds a transaction for a specific amount. This allows partial
// refunds if desired. It is only available to admins.
func (a *API) PaymentRefund(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	db := a.DB(r)
	config := gcontext.GetConfig(ctx)
	params := PaymentParams{Currency: "USD"}
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		return badRequestError("Could not read params: %v", err)
	}

	payID := chi.URLParam(r, "payment_id")
	trans, httpErr := getTransaction(db, payID)
	if httpErr != nil {
		return httpErr
	}

	if trans.Currency != params.Currency {
		return badRequestError("Currencies do not match - %v vs %v", trans.Currency, params.Currency)
	}

	if params.Amount <= 0 || params.Amount > trans.Amount {
		return badRequestError("The balance of the refund must be between 0 and the total amount")
	}

	if trans.FailureCode != "" {
		return badRequestError("Can't refund a failed transaction")
	}

	if trans.Status != models.PaidState {
		return badRequestError("Can't refund a transaction that hasn't been paid")
	}

	log := getLogEntry(r)
	order, httpErr := queryForOrder(db, trans.OrderID, log)
	if httpErr != nil {
		return httpErr
	}
	if order.PaymentProcessor == "" {
		return badRequestError("Order does not specify a payment provider")
	}

	provider := gcontext.GetPaymentProviders(ctx)[order.PaymentProcessor]
	if provider == nil {
		return badRequestError("Payment provider '%s' not configured", order.PaymentProcessor)
	}
	refund, err := provider.NewRefunder(ctx, r, log.WithField("component", "payment_provider"))
	if err != nil {
		return badRequestError("Error creating payment provider: %v", err)
	}

	// ok make the refund
	m := &models.Transaction{
		InstanceID: order.InstanceID,
		ID:         uuid.NewRandom().String(),
		Amount:     params.Amount,
		Currency:   params.Currency,
		UserID:     trans.UserID,
		OrderID:    trans.OrderID,
		Type:       models.RefundTransactionType,
		Status:     models.PendingState,
	}

	tx := db.Begin()
	tx.Create(m)
	provID := provider.Name()
	log.Debugf("Starting refund to %s", provID)
	refundID, err := refund(trans.ProcessorID, params.Amount, params.Currency)
	if err != nil {
		log.WithError(err).Info("Failed to refund value")
		m.FailureCode = strconv.FormatInt(http.StatusInternalServerError, 10)
		m.FailureDescription = err.Error()
		m.Status = models.FailedState
	} else {
		m.ProcessorID = refundID
		m.Status = models.PaidState
	}

	log.Infof("Finished transaction with %s: %s", provID, m.ProcessorID)
	tx.Save(m)
	if config.Webhooks.Refund != "" {
		hook, err := models.NewHook("refund", config.SiteURL, config.Webhooks.Refund, m.UserID, config.Webhooks.Secret, m)
		if err != nil {
			log.WithError(err).Error("Failed to process webhook")
		}
		tx.Save(hook)
	}
	tx.Commit()
	return sendJSON(w, http.StatusOK, m)
}

// PreauthorizePayment creates a new payment that can be authorized in the browser
func (a *API) PreauthorizePayment(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := getLogEntry(r)
	params := PaymentParams{}
	ct := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return badRequestError("Invalid Content-Type: %s", ct)
	}

	switch mediaType {
	case "application/json":
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			return badRequestError("Could not read params: %v", err)
		}
	case "application/x-www-form-urlencoded":
		amt, err := strconv.ParseUint(r.FormValue("amount"), 10, 64)
		if err != nil {
			return badRequestError("Error parsing amount: %v", err)
		}
		params.ProviderType = r.FormValue("provider")
		params.Amount = amt
		params.Currency = r.FormValue("currency")
		params.Description = r.FormValue("description")
	default:
		return badRequestError("Unsupported Content-Type: %s", ct)
	}

	providerType := strings.ToLower(params.ProviderType)
	if providerType == "" {
		return badRequestError("Preauthorizing a payment requires specifying a 'provider'")
	}

	provider := gcontext.GetPaymentProviders(ctx)[providerType]
	if provider == nil {
		return badRequestError("Payment provider '%s' not configured", providerType)
	}
	preauthorize, err := provider.NewPreauthorizer(ctx, r, log.WithField("component", "payment_provider"))
	if err != nil {
		return badRequestError("Error creating payment provider: %v", err)
	}

	paymentResult, err := preauthorize(params.Amount, params.Currency, params.Description)
	if err != nil {
		return internalServerError("Error preauthorizing payment: %v", err).WithInternalError(err)
	}

	return sendJSON(w, http.StatusOK, paymentResult)
}

// ------------------------------------------------------------------------------------------------
// Helpers
// ------------------------------------------------------------------------------------------------
func getTransaction(db *gorm.DB, payID string) (*models.Transaction, *HTTPError) {
	trans, err := models.GetTransaction(db, payID)
	if err != nil {
		return nil, internalServerError("Error while querying for transactions").WithInternalError(err)
	}
	if trans == nil {
		return nil, notFoundError("Transaction not found")
	}
	return trans, nil
}

func (a *API) verifyAmount(ctx context.Context, order *models.Order, amount uint64) error {
	if order.Total != amount {
		return fmt.Errorf("Amount calculated for order didn't match amount to charge. %v vs %v", order.Total, amount)
	}

	return nil
}

func queryForOrder(db *gorm.DB, orderID string, log logrus.FieldLogger) (*models.Order, *HTTPError) {
	order := &models.Order{}
	if rsp := db.Preload("Transactions").Find(order, "id = ?", orderID); rsp.Error != nil {
		if rsp.RecordNotFound() {
			return nil, notFoundError("Order not found")
		}
		return nil, internalServerError("Error while querying for order").WithInternalError(rsp.Error)
	}
	return order, nil
}

func queryForTransactions(db *gorm.DB, log logrus.FieldLogger, clause, id string) ([]models.Transaction, *HTTPError) {
	params := []interface{}{}
	if clause != "" {
		params = []interface{}{clause, id}
	}

	trans := []models.Transaction{}
	if rsp := db.Find(&trans, params...); rsp.Error != nil {
		if rsp.RecordNotFound() {
			return nil, notFoundError("Transactions not found")
		}
		return nil, internalServerError("Error while querying for transactions").WithInternalError(rsp.Error)
	}

	return trans, nil
}

// createPaymentProviders creates instance(s) of Provider based on the configuration
// provided.
func createPaymentProviders(c *conf.Configuration) (map[string]payments.Provider, error) {
	provs := map[string]payments.Provider{}
	if c.Payment.Stripe.Enabled {
		p, err := stripe.NewPaymentProvider(stripe.Config{
			SecretKey: c.Payment.Stripe.SecretKey,
		})
		if err != nil {
			return nil, err
		}
		provs[p.Name()] = p
	}
	if c.Payment.PayPal.Enabled {
		p, err := paypal.NewPaymentProvider(paypal.Config{
			Env:      c.Payment.PayPal.Env,
			ClientID: c.Payment.PayPal.ClientID,
			Secret:   c.Payment.PayPal.Secret,
		})
		if err != nil {
			return nil, err
		}
		provs[p.Name()] = p
	}
	return provs, nil
}
