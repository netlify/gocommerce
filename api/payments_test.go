package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/guregu/kami"
	"github.com/stretchr/testify/assert"

	"errors"

	"fmt"

	"context"

	"github.com/jinzhu/gorm"
	"github.com/netlify/netlify-commerce/models"
)

// ------------------------------------------------------------------------------------------------
// List by ORDER
// ------------------------------------------------------------------------------------------------

func TestPaymentsOrderForAllAsOwner(t *testing.T) {
	db, config := db(t)

	ctx := testContext(testToken(testUser.ID, ""), config, false)
	ctx = kami.SetParam(ctx, "order_id", firstOrder.ID)
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", nil)

	NewAPI(config, db, nil).PaymentListForOrder(ctx, w, r)

	// we should have gotten back a list of transactions
	trans := []models.Transaction{}
	extractPayload(t, 200, w, &trans)
	assert.Equal(t, 1, len(trans))
	validateTransaction(t, firstTransaction, &trans[0])
}

func TestPaymentsOrderQueryForAllAsAdmin(t *testing.T) {
	db, config := db(t)
	anotherTransaction := models.NewTransaction(firstOrder)
	db.Create(anotherTransaction)
	defer db.Unscoped().Delete(anotherTransaction)

	ctx := testContext(testToken("magical-unicorn", ""), config, true)
	ctx = kami.SetParam(ctx, "order_id", firstOrder.ID)
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", nil)

	NewAPI(config, db, nil).PaymentListForOrder(ctx, w, r)

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
	db, config := db(t)

	ctx := testContext(nil, config, false)
	ctx = kami.SetParam(ctx, "order_id", firstOrder.ID)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", nil)
	NewAPI(config, db, nil).PaymentListForOrder(ctx, w, r)

	// should get a 401 ~ claims are required
	validateError(t, 401, w)
}

// ------------------------------------------------------------------------------------------------
// List by USER
// ------------------------------------------------------------------------------------------------
func TestPaymentsUserForAllAsUser(t *testing.T) {
	db, config := db(t)

	ctx := testContext(testToken(testUser.ID, ""), config, false)
	ctx = kami.SetParam(ctx, "user_id", testUser.ID)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", nil)
	NewAPI(config, db, nil).PaymentListForUser(ctx, w, r)

	actual := []models.Transaction{}
	extractPayload(t, 200, w, &actual)
	validateAllTransactions(t, actual)
}

func TestPaymentsUserForAllAsAdmin(t *testing.T) {
	db, config := db(t)

	ctx := testContext(testToken("magical-unicorn", ""), config, true)
	ctx = kami.SetParam(ctx, "user_id", testUser.ID)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", nil)
	NewAPI(config, db, nil).PaymentListForUser(ctx, w, r)

	actual := []models.Transaction{}
	extractPayload(t, 200, w, &actual)

	validateAllTransactions(t, actual)
}

func TestPaymentsUserForAllAsAnon(t *testing.T) {
	db, config := db(t)

	ctx := testContext(nil, config, false)
	ctx = kami.SetParam(ctx, "user_id", testUser.ID)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", nil)
	NewAPI(config, db, nil).PaymentListForUser(ctx, w, r)

	// should get a 401 ~ claims are required
	validateError(t, 401, w)
}

func TestPaymentsUserForAllAsStranger(t *testing.T) {
	db, config := db(t)

	ctx := testContext(testToken("stranger-danger", ""), config, false)
	ctx = kami.SetParam(ctx, "user_id", testUser.ID)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", nil)
	NewAPI(config, db, nil).PaymentListForUser(ctx, w, r)

	// should get a 401 ~ not the right user
	validateError(t, 401, w)
}

// ------------------------------------------------------------------------------------------------
// List with params
// ------------------------------------------------------------------------------------------------
func TestPaymentsListAllAsNonAdmin(t *testing.T) {
	config := testConfig()
	ctx := testContext(testToken("stranger-danger", ""), config, false)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", nil)
	NewAPI(config, nil, nil).PaymentList(ctx, w, r)

	// should get a 401 ~ not the right user
	validateError(t, 401, w)
}

func TestPaymentsListWithParams(t *testing.T) {
	db, config := db(t)

	ctx := testContext(testToken("magical-unicorn", ""), config, true)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something?processor_id=stripe", nil)
	NewAPI(config, db, nil).PaymentList(ctx, w, r)

	trans := []models.Transaction{}
	extractPayload(t, 200, w, &trans)

	assert.Equal(t, 1, len(trans))
	validateTransaction(t, firstTransaction, &trans[0])
}

func TestPaymentsListNoParams(t *testing.T) {
	db, config := db(t)

	ctx := testContext(testToken("magical-unicorn", ""), config, true)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", nil)
	NewAPI(config, db, nil).PaymentList(ctx, w, r)

	trans := []models.Transaction{}
	extractPayload(t, 200, w, &trans)

	validateAllTransactions(t, trans)
}

func TestPaymentsViewAsNonAdmin(t *testing.T) {
	config := testConfig()
	ctx := testContext(testToken("stranger-danger", ""), config, false)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", nil)
	NewAPI(config, nil, nil).PaymentView(ctx, w, r)

	// should get a 401 ~ not the right user
	validateError(t, 401, w)
}

func TestPaymentsView(t *testing.T) {
	db, config := db(t)

	ctx := testContext(testToken("magical-unicorn", ""), config, true)
	ctx = kami.SetParam(ctx, "pay_id", firstTransaction.ID)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", nil)
	NewAPI(config, db, nil).PaymentView(ctx, w, r)

	trans := new(models.Transaction)
	extractPayload(t, 200, w, trans)

	validateTransaction(t, firstTransaction, trans)
}

func TestPaymentsViewMissingPayment(t *testing.T) {
	db, config := db(t)

	ctx := testContext(testToken("magical-unicorn", ""), config, true)
	ctx = kami.SetParam(ctx, "pay_id", "nonsense")

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", nil)
	NewAPI(config, db, nil).PaymentView(ctx, w, r)

	validateError(t, 404, w)
}

func TestPaymentsRefundMismatchedCurrency(t *testing.T) {
	w, _ := runPaymentRefund(t, &PaymentParams{
		Amount:      1,
		Currency:    "monopoly-money",
		StripeToken: "123",
	})

	validateError(t, 400, w)
}

func TestPaymentsRefundMissingStripeToken(t *testing.T) {
	w, _ := runPaymentRefund(t, &PaymentParams{
		Amount:      1,
		Currency:    firstTransaction.ID,
		StripeToken: "",
	})
	validateError(t, 400, w)
}

func TestPaymentsRefundAmountTooHighOrLow(t *testing.T) {
	w, _ := runPaymentRefund(t, &PaymentParams{
		Amount:      1000,
		Currency:    firstTransaction.Currency,
		StripeToken: "123",
	})

	validateError(t, 400, w)
}

func TestPaymentsRefundUnknownPayment(t *testing.T) {
	db, config := db(t)
	ctx := testContext(testToken("magical-unicorn", ""), config, true)
	ctx = kami.SetParam(ctx, "pay_id", "nothign")

	params := &PaymentParams{
		Amount:      1,
		Currency:    firstTransaction.Currency,
		StripeToken: "123",
	}

	body, _ := json.Marshal(params)
	r, _ := http.NewRequest("POST", "http://something", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	NewAPI(config, db, nil).PaymentRefund(ctx, w, r)

	validateError(t, 404, w)
}

func TestPaymentsRefundUnpaid(t *testing.T) {
	db, config := db(t)
	firstTransaction.Status = models.PendingState
	db.Save(firstTransaction)

	ctx := testContext(testToken("magical-unicorn", ""), config, true)
	ctx = kami.SetParam(ctx, "pay_id", firstTransaction.ID)
	ctx = context.WithValue(ctx, payerKey, nil)

	params := &PaymentParams{
		Amount:      1,
		Currency:    firstTransaction.Currency,
		StripeToken: "123",
	}

	body, _ := json.Marshal(params)
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "http://something", bytes.NewBuffer(body))

	NewAPI(config, db, nil).PaymentRefund(ctx, w, r)

	validateError(t, 400, w)
}

func runPaymentRefund(t *testing.T, params *PaymentParams) (*httptest.ResponseRecorder, *gorm.DB) {
	db, config := db(t)
	ctx := testContext(testToken("magical-unicorn", ""), config, true)
	ctx = kami.SetParam(ctx, "pay_id", firstTransaction.ID)
	ctx = context.WithValue(ctx, payerKey, nil)

	body, _ := json.Marshal(params)
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "http://something", bytes.NewBuffer(body))

	NewAPI(config, db, nil).PaymentRefund(ctx, w, r)
	return w, db
}

func TestPaymentsRefundSuccess(t *testing.T) {
	db, config := db(t)
	provider := &memProvider{}
	ctx := testContext(testToken("magical-unicorn", ""), config, true)
	ctx = kami.SetParam(ctx, "pay_id", firstTransaction.ID)
	ctx = context.WithValue(ctx, payerKey, provider)

	params := &PaymentParams{
		Amount:      1,
		Currency:    firstTransaction.Currency,
		StripeToken: "123",
	}
	body, _ := json.Marshal(params)
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "http://something", bytes.NewBuffer(body))
	NewAPI(config, db, nil).PaymentRefund(ctx, w, r)

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

type memProvider struct {
	refundCalls []refundCall
}

type refundCall struct {
	amount uint64
	id     string
}

func (mp *memProvider) charge(amount uint64, currency, token string) (string, error) {
	return "", errors.New("Shouldn't have called this")
}

func (mp *memProvider) refund(amount uint64, id string) (string, error) {
	if mp.refundCalls == nil {
		mp.refundCalls = []refundCall{}
	}
	mp.refundCalls = append(mp.refundCalls, refundCall{
		amount: amount,
		id:     id,
	})

	return fmt.Sprintf("trans-%d", len(mp.refundCalls)), nil
}
