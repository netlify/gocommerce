package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"errors"

	"fmt"

	"github.com/jinzhu/gorm"
	gcontext "github.com/netlify/gocommerce/context"
	"github.com/netlify/gocommerce/models"
	"github.com/netlify/gocommerce/payments"
)

// ------------------------------------------------------------------------------------------------
// List by ORDER
// ------------------------------------------------------------------------------------------------

func TestPaymentsOrderForAllAsOwner(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(nil, config, false)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", urlForFirstOrder+"/payments", nil).WithContext(ctx)

	token := testToken(testUser.ID, "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(w, r)

	// we should have gotten back a list of transactions
	trans := []models.Transaction{}
	extractPayload(t, http.StatusOK, w, &trans)
	assert.Equal(t, 1, len(trans))
	validateTransaction(t, firstTransaction, &trans[0])
}

func TestPaymentsOrderQueryForAllAsAdmin(t *testing.T) {
	db, globalConfig, config := db(t)
	anotherTransaction := models.NewTransaction(firstOrder)
	db.Create(anotherTransaction)
	defer db.Unscoped().Delete(anotherTransaction)

	ctx := testContext(nil, config, true)
	chi.RouteContext(ctx).URLParams.Add("order_id", firstOrder.ID)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", urlForFirstOrder+"/payments", nil).WithContext(ctx)

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(w, r)

	// we should have gotten back a list of transactions
	trans := []models.Transaction{}
	extractPayload(t, http.StatusOK, w, &trans)
	assert.Equal(t, 2, len(trans))
	for _, tran := range trans {
		switch tran.ID {
		case anotherTransaction.ID:
			validateTransaction(t, anotherTransaction, &tran)
		case firstTransaction.ID:
			validateTransaction(t, firstTransaction, &tran)
		default:
			assert.Fail(t, "Unknown transaction: "+tran.ID)
		}
	}
}

func TestPaymentsOrderQueryForAllAsAnon(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(nil, config, false)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", urlForFirstOrder+"/payments", nil).WithContext(ctx)

	NewAPI(globalConfig, config, db).handler.ServeHTTP(w, r)

	// should get a 401 ~ claims are required
	validateError(t, http.StatusUnauthorized, w)
}

// ------------------------------------------------------------------------------------------------
// List by USER
// ------------------------------------------------------------------------------------------------
func TestPaymentsUserForAllAsUser(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(nil, config, false)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "https://example.com/users/"+testUser.ID+"/payments", nil).WithContext(ctx)

	token := testToken(testUser.ID, "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(w, r)

	actual := []models.Transaction{}
	extractPayload(t, http.StatusOK, w, &actual)
	validateAllTransactions(t, actual)
}

func TestPaymentsUserForAllAsAdmin(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(nil, config, true)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "https://example.com/users/"+testUser.ID+"/payments", nil).WithContext(ctx)

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(w, r)

	actual := []models.Transaction{}
	extractPayload(t, http.StatusOK, w, &actual)

	validateAllTransactions(t, actual)
}

func TestPaymentsUserForAllAsAnon(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(nil, config, false)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "https://example.com/users/"+testUser.ID+"/payments", nil).WithContext(ctx)
	NewAPI(globalConfig, config, db).handler.ServeHTTP(w, r)

	// should get a 401 ~ claims are required
	validateError(t, http.StatusUnauthorized, w)
}

func TestPaymentsUserForAllAsStranger(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(nil, config, false)
	chi.RouteContext(ctx).URLParams.Add("user_id", testUser.ID)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "https://example.com/users/"+testUser.ID+"/payments", nil).WithContext(ctx)

	token := testToken("stranger-danger", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(w, r)

	// should get a 401 ~ not the right user
	validateError(t, http.StatusUnauthorized, w)
}

// ------------------------------------------------------------------------------------------------
// List with params
// ------------------------------------------------------------------------------------------------
func TestPaymentsListAllAsNonAdmin(t *testing.T) {
	globalConfig, config := testConfig()
	ctx := testContext(nil, config, false)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "https://example.com/payments", nil).WithContext(ctx)

	token := testToken("stranger-danger", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, nil).handler.ServeHTTP(w, r)

	// should get a 401 ~ not the right user
	validateError(t, http.StatusUnauthorized, w)
}

func TestPaymentsListWithParams(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(nil, config, true)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "https://example.com/payments?processor_id=stripe", nil).WithContext(ctx)

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(w, r)

	trans := []models.Transaction{}
	extractPayload(t, http.StatusOK, w, &trans)

	assert.Equal(t, 1, len(trans))
	validateTransaction(t, firstTransaction, &trans[0])
}

func TestPaymentsListNoParams(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(nil, config, true)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "https://example.com/payments", nil).WithContext(ctx)

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(w, r)

	trans := []models.Transaction{}
	extractPayload(t, http.StatusOK, w, &trans)

	validateAllTransactions(t, trans)
}

func TestPaymentsViewAsNonAdmin(t *testing.T) {
	globalConfig, config := testConfig()
	ctx := testContext(nil, config, false)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://example.com/payments/123", nil).WithContext(ctx)

	token := testToken("stranger-danger", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, nil).handler.ServeHTTP(w, r)

	// should get a 401 ~ not the right user
	validateError(t, http.StatusUnauthorized, w)
}

func TestPaymentsView(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(nil, config, true)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://example.com/payments/"+firstTransaction.ID, nil).WithContext(ctx)

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(w, r)

	trans := new(models.Transaction)
	extractPayload(t, http.StatusOK, w, trans)

	validateTransaction(t, firstTransaction, trans)
}

func TestPaymentsViewMissingPayment(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(testToken("magical-unicorn", ""), config, true)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://example.com/payments/nonsense", nil).WithContext(ctx)

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(w, r)

	validateError(t, http.StatusNotFound, w, "Transaction not found")
}

func TestPaymentsRefundMismatchedCurrency(t *testing.T) {
	w, _ := runPaymentRefund(t, &PaymentParams{
		Amount:   1,
		Currency: "monopoly-money",
	})

	validateError(t, http.StatusBadRequest, w, "Currencies do not match")
}

func TestPaymentsRefundAmountTooHighOrLow(t *testing.T) {
	w, _ := runPaymentRefund(t, &PaymentParams{
		Amount:   1000,
		Currency: "usd",
	})

	validateError(t, http.StatusBadRequest, w, "must be between 0 and the total amount")
}

func TestPaymentsRefundPaypal(t *testing.T) {
	var loginCount, refundCount int
	refundID := "4CF18861HF410323U"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/oauth2/token":
			w.Header().Add("Content-Type", "application/json")
			fmt.Fprint(w, `{"access_token":"EEwJ6tF9x5WCIZDYzyZGaz6Khbw7raYRIBV_WxVvgmsG","expires_in":100000}`)
			loginCount++
		case "/v1/payments/sale/" + secondTransaction.ProcessorID + "/refund":
			w.Header().Add("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"`+refundID+`"}`)
			refundCount++
		default:
			w.WriteHeader(500)
			t.Fatalf("unknown PayPal API call to %s", r.URL.Path)
		}
	}))
	defer server.Close()

	db, globalConfig, config := db(t)
	config.Payment.PayPal.Enabled = true
	config.Payment.PayPal.ClientID = "clientid"
	config.Payment.PayPal.Secret = "secret"
	config.Payment.PayPal.Env = server.URL
	ctx := testContext(nil, config, true)
	provs, err := createPaymentProviders(config)
	require.Nil(t, err)
	ctx = gcontext.WithPaymentProviders(ctx, provs)

	params := &paypalPaymentParams{
		Amount:       1,
		Currency:     secondTransaction.Currency,
		PaypalID:     "123",
		PaypalUserID: "456",
	}

	body, _ := json.Marshal(params)
	r := httptest.NewRequest("POST", "http://example.com/payments/"+secondTransaction.ID+"/refund", bytes.NewBuffer(body)).WithContext(ctx)
	w := httptest.NewRecorder()

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(w, r)

	rsp := models.Transaction{}
	extractPayload(t, http.StatusOK, w, &rsp)
	assert.Equal(t, refundID, rsp.ProcessorID)
	// this is 2 because we manually create a PayPal provider to pass in our test context.
	assert.Equal(t, 2, loginCount, "too many login calls")
	assert.Equal(t, 1, refundCount, "too many refund calls")
}

func TestPaymentsRefundUnknownPayment(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(testToken("magical-unicorn", ""), config, true)

	params := &stripePaymentParams{
		Amount:      1,
		Currency:    firstTransaction.Currency,
		StripeToken: "123",
	}

	body, _ := json.Marshal(params)
	r := httptest.NewRequest("POST", "http://example.com/payments/nothing/refund", bytes.NewBuffer(body)).WithContext(ctx)
	w := httptest.NewRecorder()

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(w, r)
	validateError(t, http.StatusNotFound, w)
}

func TestPaymentsRefundUnpaid(t *testing.T) {
	db, globalConfig, config := db(t)
	firstTransaction.Status = models.PendingState
	db.Save(firstTransaction)

	ctx := testContext(nil, config, true)
	ctx = gcontext.WithPaymentProviders(ctx, nil)

	params := &stripePaymentParams{
		Amount:      1,
		Currency:    firstTransaction.Currency,
		StripeToken: "123",
	}

	body, _ := json.Marshal(params)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "http://example.com/payments/"+firstTransaction.ID+"/refund", bytes.NewBuffer(body)).WithContext(ctx)

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(w, r)
	validateError(t, http.StatusBadRequest, w)
}

func runPaymentRefund(t *testing.T, params *PaymentParams) (*httptest.ResponseRecorder, *gorm.DB) {
	db, globalConfig, config := db(t)
	ctx := testContext(nil, config, true)
	ctx = gcontext.WithPaymentProviders(ctx, nil)

	body, _ := json.Marshal(params)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "http://example.com/payments/"+firstTransaction.ID+"/refund", bytes.NewBuffer(body)).WithContext(ctx)

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(w, r)
	return w, db
}

func TestPaymentsRefundSuccess(t *testing.T) {
	db, globalConfig, config := db(t)
	// unused, but needed to pass safety check
	config.Payment.Stripe.Enabled = true
	config.Payment.Stripe.SecretKey = "secret"

	provider := &memProvider{name: payments.StripeProvider}
	ctx := testContext(nil, config, true)
	ctx, err := withTenantConfig(ctx, config)
	require.NoError(t, err)
	ctx = gcontext.WithPaymentProviders(ctx, map[string]payments.Provider{payments.StripeProvider: provider})

	params := &stripePaymentParams{
		Amount:      1,
		Currency:    firstTransaction.Currency,
		StripeToken: "123",
	}
	body, _ := json.Marshal(params)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "http://example.com/payments/"+firstTransaction.ID+"/refund", bytes.NewBuffer(body))

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPIWithVersion(ctx, globalConfig, db, defaultVersion).handler.ServeHTTP(w, r)

	rsp := new(models.Transaction)
	extractPayload(t, http.StatusOK, w, rsp)

	stored := &models.Transaction{ID: rsp.ID}
	db.First(stored)

	for _, payment := range []*models.Transaction{stored, rsp} {
		assert.NotEmpty(t, payment.ID)
		assert.Equal(t, testUser.ID, payment.UserID)
		assert.EqualValues(t, 1, payment.Amount)
		assert.Equal(t, "usd", payment.Currency)
		assert.Empty(t, payment.FailureCode)
		assert.Empty(t, payment.FailureDescription)
		assert.Equal(t, models.RefundTransactionType, payment.Type)
		assert.Equal(t, models.PaidState, payment.Status)
	}
}

// ------------------------------------------------------------------------------------------------
// Validators
// ------------------------------------------------------------------------------------------------
func validateTransaction(t *testing.T, expected *models.Transaction, actual *models.Transaction) {
	assert := assert.New(t)
	assert.Equal(expected.Currency, actual.Currency)
	assert.Equal(expected.ID, actual.ID)
	assert.Equal(expected.OrderID, actual.OrderID)
	assert.Equal(expected.ProcessorID, actual.ProcessorID)
	assert.Equal(expected.UserID, actual.UserID)
	assert.Equal(expected.Amount, actual.Amount)
	assert.Equal(expected.Currency, actual.Currency)
	assert.Equal(expected.FailureCode, actual.FailureCode)
	assert.Equal(expected.FailureDescription, actual.FailureDescription)
	assert.Equal(expected.Status, actual.Status)
	assert.Equal(expected.Type, actual.Type)
	assert.Equal(expected.CreatedAt.UTC(), actual.CreatedAt.UTC())
}

func validateAllTransactions(t *testing.T, trans []models.Transaction) {
	assert.Equal(t, 2, len(trans))
	for _, tran := range trans {
		switch tran.ID {
		case secondTransaction.ID:
			validateTransaction(t, secondTransaction, &tran)
		case firstTransaction.ID:
			validateTransaction(t, firstTransaction, &tran)
		default:
			assert.Fail(t, "Unknown transaction: "+tran.ID)
		}
	}
}

type stripePaymentParams struct {
	Amount      uint64
	Currency    string
	StripeToken string `json:"stripe_token"`
}

type paypalPaymentParams struct {
	Amount       uint64
	Currency     string
	PaypalID     string
	PaypalUserID string
}

type memProvider struct {
	refundCalls []refundCall
	name        string
}

type refundCall struct {
	amount   uint64
	id       string
	currency string
}

func (mp *memProvider) Name() string {
	return mp.name
}
func (mp *memProvider) NewCharger(ctx context.Context, r *http.Request) (payments.Charger, error) {
	return mp.charge, nil
}
func (mp *memProvider) NewRefunder(ctx context.Context, r *http.Request) (payments.Refunder, error) {
	return mp.refund, nil
}
func (mp *memProvider) NewPreauthorizer(ctx context.Context, r *http.Request) (payments.Preauthorizer, error) {
	return mp.preauthorize, nil
}

func (mp *memProvider) charge(amount uint64, currency string) (string, error) {
	return "", errors.New("Shouldn't have called this")
}

func (mp *memProvider) refund(transactionID string, amount uint64, currency string) (string, error) {
	if mp.refundCalls == nil {
		mp.refundCalls = []refundCall{}
	}
	mp.refundCalls = append(mp.refundCalls, refundCall{
		amount:   amount,
		id:       transactionID,
		currency: currency,
	})

	return fmt.Sprintf("trans-%d", len(mp.refundCalls)), nil
}

func (mp *memProvider) preauthorize(amount uint64, currency string, description string) (*payments.PreauthorizationResult, error) {
	return nil, nil
}
