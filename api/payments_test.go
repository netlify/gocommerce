package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/guregu/kami"
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

	ctx := testContext(testToken(testUser.ID, ""), config, false)
	ctx = kami.SetParam(ctx, "order_id", firstOrder.ID)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://something", nil)

	NewAPI(globalConfig, config, db).PaymentListForOrder(ctx, w, r)

	// we should have gotten back a list of transactions
	trans := []models.Transaction{}
	extractPayload(t, 200, w, &trans)
	assert.Equal(t, 1, len(trans))
	validateTransaction(t, firstTransaction, &trans[0])
}

func TestPaymentsOrderQueryForAllAsAdmin(t *testing.T) {
	db, globalConfig, config := db(t)
	anotherTransaction := models.NewTransaction(firstOrder)
	db.Create(anotherTransaction)
	defer db.Unscoped().Delete(anotherTransaction)

	ctx := testContext(testToken("magical-unicorn", ""), config, true)
	ctx = kami.SetParam(ctx, "order_id", firstOrder.ID)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://something", nil)

	NewAPI(globalConfig, config, db).PaymentListForOrder(ctx, w, r)

	// we should have gotten back a list of transactions
	trans := []models.Transaction{}
	extractPayload(t, 200, w, &trans)
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
	ctx = kami.SetParam(ctx, "order_id", firstOrder.ID)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://something", nil)
	NewAPI(globalConfig, config, db).PaymentListForOrder(ctx, w, r)

	// should get a 401 ~ claims are required
	validateError(t, 401, w)
}

// ------------------------------------------------------------------------------------------------
// List by USER
// ------------------------------------------------------------------------------------------------
func TestPaymentsUserForAllAsUser(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(testToken(testUser.ID, ""), config, false)
	ctx = kami.SetParam(ctx, "user_id", testUser.ID)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://something", nil)
	NewAPI(globalConfig, config, db).PaymentListForUser(ctx, w, r)

	actual := []models.Transaction{}
	extractPayload(t, 200, w, &actual)
	validateAllTransactions(t, actual)
}

func TestPaymentsUserForAllAsAdmin(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(testToken("magical-unicorn", ""), config, true)
	ctx = kami.SetParam(ctx, "user_id", testUser.ID)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://something", nil)
	NewAPI(globalConfig, config, db).PaymentListForUser(ctx, w, r)

	actual := []models.Transaction{}
	extractPayload(t, 200, w, &actual)

	validateAllTransactions(t, actual)
}

func TestPaymentsUserForAllAsAnon(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(nil, config, false)
	ctx = kami.SetParam(ctx, "user_id", testUser.ID)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://something", nil)
	NewAPI(globalConfig, config, db).PaymentListForUser(ctx, w, r)

	// should get a 401 ~ claims are required
	validateError(t, 401, w)
}

func TestPaymentsUserForAllAsStranger(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(testToken("stranger-danger", ""), config, false)
	ctx = kami.SetParam(ctx, "user_id", testUser.ID)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://something", nil)
	NewAPI(globalConfig, config, db).PaymentListForUser(ctx, w, r)

	// should get a 401 ~ not the right user
	validateError(t, 401, w)
}

// ------------------------------------------------------------------------------------------------
// List with params
// ------------------------------------------------------------------------------------------------
func TestPaymentsListAllAsNonAdmin(t *testing.T) {
	globalConfig, config := testConfig()
	ctx := testContext(testToken("stranger-danger", ""), config, false)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://something", nil)
	NewAPI(globalConfig, config, nil).PaymentList(ctx, w, r)

	// should get a 401 ~ not the right user
	validateError(t, 401, w)
}

func TestPaymentsListWithParams(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(testToken("magical-unicorn", ""), config, true)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://something?processor_id=stripe", nil)
	NewAPI(globalConfig, config, db).PaymentList(ctx, w, r)

	trans := []models.Transaction{}
	extractPayload(t, 200, w, &trans)

	assert.Equal(t, 1, len(trans))
	validateTransaction(t, firstTransaction, &trans[0])
}

func TestPaymentsListNoParams(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(testToken("magical-unicorn", ""), config, true)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://something", nil)
	NewAPI(globalConfig, config, db).PaymentList(ctx, w, r)

	trans := []models.Transaction{}
	extractPayload(t, 200, w, &trans)

	validateAllTransactions(t, trans)
}

func TestPaymentsViewAsNonAdmin(t *testing.T) {
	globalConfig, config := testConfig()
	ctx := testContext(testToken("stranger-danger", ""), config, false)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://something", nil)
	NewAPI(globalConfig, config, nil).PaymentView(ctx, w, r)

	// should get a 401 ~ not the right user
	validateError(t, 401, w)
}

func TestPaymentsView(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(testToken("magical-unicorn", ""), config, true)
	ctx = kami.SetParam(ctx, "pay_id", firstTransaction.ID)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://something", nil)
	NewAPI(globalConfig, config, db).PaymentView(ctx, w, r)

	trans := new(models.Transaction)
	extractPayload(t, 200, w, trans)

	validateTransaction(t, firstTransaction, trans)
}

func TestPaymentsViewMissingPayment(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(testToken("magical-unicorn", ""), config, true)
	ctx = kami.SetParam(ctx, "pay_id", "nonsense")

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://something", nil)
	NewAPI(globalConfig, config, db).PaymentView(ctx, w, r)

	validateError(t, 404, w, "Transaction not found")
}

func TestPaymentsRefundMismatchedCurrency(t *testing.T) {
	w, _ := runPaymentRefund(t, &PaymentParams{
		Amount:   1,
		Currency: "monopoly-money",
	})

	validateError(t, 400, w, "Currencies do not match")
}

func TestPaymentsRefundAmountTooHighOrLow(t *testing.T) {
	w, _ := runPaymentRefund(t, &PaymentParams{
		Amount:   1000,
		Currency: "usd",
	})

	validateError(t, 400, w, "must be between 0 and the total amount")
}

func TestPaymentsRefundPaypal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"EEwJ6tF9x5WCIZDYzyZGaz6Khbw7raYRIBV_WxVvgmsG","expires_in":100000}`)
	}))
	defer server.Close()

	db, globalConfig, config := db(t)
	config.Payment.ProviderType = payments.PayPalProvider
	config.Payment.PayPal.ClientID = "clientid"
	config.Payment.PayPal.Secret = "secret"
	config.Payment.PayPal.Env = server.URL
	ctx := testContext(testToken("magical-unicorn", ""), config, true)
	ctx = kami.SetParam(ctx, "pay_id", secondTransaction.ID)
	prov, err := createPaymentProvider(config)
	require.Nil(t, err)
	ctx = gcontext.WithPaymentProvider(ctx, prov)

	params := &paypalPaymentParams{
		Amount:       1,
		Currency:     secondTransaction.Currency,
		PaypalID:     "123",
		PaypalUserID: "456",
	}

	body, _ := json.Marshal(params)
	r := httptest.NewRequest("POST", "http://something", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	NewAPI(globalConfig, config, db).PaymentRefund(ctx, w, r)

	validateError(t, 400, w, "does not support refunds")
}

func TestPaymentsRefundUnknownPayment(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(testToken("magical-unicorn", ""), config, true)
	ctx = kami.SetParam(ctx, "pay_id", "nothign")

	params := &stripePaymentParams{
		Amount:      1,
		Currency:    firstTransaction.Currency,
		StripeToken: "123",
	}

	body, _ := json.Marshal(params)
	r := httptest.NewRequest("POST", "http://something", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	NewAPI(globalConfig, config, db).PaymentRefund(ctx, w, r)

	validateError(t, 404, w)
}

func TestPaymentsRefundUnpaid(t *testing.T) {
	db, globalConfig, config := db(t)
	firstTransaction.Status = models.PendingState
	db.Save(firstTransaction)

	ctx := testContext(testToken("magical-unicorn", ""), config, true)
	ctx = kami.SetParam(ctx, "pay_id", firstTransaction.ID)
	ctx = gcontext.WithPaymentProvider(ctx, nil)

	params := &stripePaymentParams{
		Amount:      1,
		Currency:    firstTransaction.Currency,
		StripeToken: "123",
	}

	body, _ := json.Marshal(params)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "http://something", bytes.NewBuffer(body))

	NewAPI(globalConfig, config, db).PaymentRefund(ctx, w, r)

	validateError(t, 400, w)
}

func runPaymentRefund(t *testing.T, params *PaymentParams) (*httptest.ResponseRecorder, *gorm.DB) {
	db, globalConfig, config := db(t)
	ctx := testContext(testToken("magical-unicorn", ""), config, true)
	ctx = kami.SetParam(ctx, "pay_id", firstTransaction.ID)
	ctx = gcontext.WithPaymentProvider(ctx, nil)

	body, _ := json.Marshal(params)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "http://something", bytes.NewBuffer(body))

	NewAPI(globalConfig, config, db).PaymentRefund(ctx, w, r)
	return w, db
}

func TestPaymentsRefundSuccess(t *testing.T) {
	db, globalConfig, config := db(t)
	provider := &memProvider{name: payments.StripeProvider}
	ctx := testContext(testToken("magical-unicorn", ""), config, true)
	ctx = kami.SetParam(ctx, "pay_id", firstTransaction.ID)
	ctx = gcontext.WithPaymentProvider(ctx, provider)

	params := &stripePaymentParams{
		Amount:      1,
		Currency:    firstTransaction.Currency,
		StripeToken: "123",
	}
	body, _ := json.Marshal(params)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "http://something", bytes.NewBuffer(body))
	NewMultiTenantAPI(globalConfig, db).PaymentRefund(ctx, w, r)

	rsp := new(models.Transaction)
	extractPayload(t, 200, w, rsp)

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
	amount uint64
	id     string
}

func (mp *memProvider) Name() string {
	return mp.name
}
func (mp *memProvider) NewCharger(ctx context.Context, r *http.Request) (payments.Charger, error) {
	return mp, nil
}
func (mp *memProvider) NewRefunder(ctx context.Context, r *http.Request) (payments.Refunder, error) {
	return mp, nil
}
func (mp *memProvider) NewPreauthorizer(ctx context.Context, r *http.Request) (payments.Preauthorizer, error) {
	return mp, nil
}

func (mp *memProvider) Charge(amount uint64, currency string) (string, error) {
	return "", errors.New("Shouldn't have called this")
}

func (mp *memProvider) Refund(transactionID string, amount uint64) (string, error) {
	if mp.refundCalls == nil {
		mp.refundCalls = []refundCall{}
	}
	mp.refundCalls = append(mp.refundCalls, refundCall{
		amount: amount,
		id:     transactionID,
	})

	return fmt.Sprintf("trans-%d", len(mp.refundCalls)), nil
}

func (mp *memProvider) Preauthorize(amount uint64, currency string, description string) (*payments.PreauthorizationResult, error) {
	return nil, nil
}
