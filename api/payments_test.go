package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"errors"

	"fmt"

	gcontext "github.com/netlify/gocommerce/context"
	"github.com/netlify/gocommerce/models"
	"github.com/netlify/gocommerce/payments"
)

// ------------------------------------------------------------------------------------------------
// List by ORDER
// ------------------------------------------------------------------------------------------------

func TestOrderPaymentsList(t *testing.T) {
	t.Run("AsOwner", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testToken(testData.testUser.ID, "")
		recorder := test.TestEndpoint(http.MethodGet, testData.urlForFirstOrder+"/payments", nil, token)

		// we should have gotten back a list of transactions
		trans := []models.Transaction{}
		extractPayload(t, http.StatusOK, recorder, &trans)
		assert.Len(t, trans, 1)
		validateTransaction(t, testData.firstTransaction, &trans[0])
	})

	t.Run("AsAdmin", func(t *testing.T) {
		test := NewRouteTest(t)
		anotherTransaction := models.NewTransaction(testData.firstOrder)
		test.DB.Create(anotherTransaction)
		defer test.DB.Unscoped().Delete(anotherTransaction)

		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodGet, testData.urlForFirstOrder+"/payments", nil, token)

		trans := []models.Transaction{}
		extractPayload(t, http.StatusOK, recorder, &trans)
		assert.Len(t, trans, 2)
		for _, tran := range trans {
			switch tran.ID {
			case anotherTransaction.ID:
				validateTransaction(t, anotherTransaction, &tran)
			case testData.firstTransaction.ID:
				validateTransaction(t, testData.firstTransaction, &tran)
			default:
				assert.Fail(t, "Unknown transaction: "+tran.ID)
			}
		}
	})

	t.Run("Anonymous", func(t *testing.T) {
		test := NewRouteTest(t)
		recorder := test.TestEndpoint(http.MethodGet, testData.urlForFirstOrder+"/payments", nil, nil)
		validateError(t, http.StatusUnauthorized, recorder)
	})
}

// ------------------------------------------------------------------------------------------------
// List by USER
// ------------------------------------------------------------------------------------------------

func TestUserPaymentsList(t *testing.T) {
	url := "/users/" + testData.testUser.ID + "/payments"

	t.Run("AsUser", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testToken(testData.testUser.ID, "")
		recorder := test.TestEndpoint(http.MethodGet, url, nil, token)

		actual := []models.Transaction{}
		extractPayload(t, http.StatusOK, recorder, &actual)
		validateAllTransactions(t, testData, actual)
	})

	t.Run("AsAdmin", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodGet, url, nil, token)

		actual := []models.Transaction{}
		extractPayload(t, http.StatusOK, recorder, &actual)
		validateAllTransactions(t, testData, actual)
	})

	t.Run("Anonymous", func(t *testing.T) {
		test := NewRouteTest(t)
		recorder := test.TestEndpoint(http.MethodGet, url, nil, nil)
		validateError(t, http.StatusUnauthorized, recorder)
	})

	t.Run("AsStranger", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testToken("stranger-danger", "")
		recorder := test.TestEndpoint(http.MethodGet, url, nil, token)
		validateError(t, http.StatusUnauthorized, recorder)
	})
}

// ------------------------------------------------------------------------------------------------
// List with params
// ------------------------------------------------------------------------------------------------
func TestPaymentsList(t *testing.T) {
	url := "/payments"

	t.Run("AsNonAdmin", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testToken("stranger-danger", "")
		recorder := test.TestEndpoint(http.MethodGet, url, nil, token)
		validateError(t, http.StatusUnauthorized, recorder)
	})

	t.Run("WithParams", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodGet, url+"?processor_id=stripe", nil, token)

		trans := []models.Transaction{}
		extractPayload(t, http.StatusOK, recorder, &trans)

		assert.Len(t, trans, 1)
		validateTransaction(t, testData.firstTransaction, &trans[0])
	})

	t.Run("NoParams", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodGet, url, nil, token)

		trans := []models.Transaction{}
		extractPayload(t, http.StatusOK, recorder, &trans)
		validateAllTransactions(t, testData, trans)
	})
}

func TestPaymentsView(t *testing.T) {
	t.Run("AsNonAdmin", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testToken("stranger-danger", "")
		recorder := test.TestEndpoint(http.MethodGet, "/payments/123", nil, token)
		validateError(t, http.StatusUnauthorized, recorder)
	})

	t.Run("AsAdmin", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodGet, "/payments/"+testData.firstTransaction.ID, nil, token)

		trans := new(models.Transaction)
		extractPayload(t, http.StatusOK, recorder, trans)
		validateTransaction(t, testData.firstTransaction, trans)
	})

	t.Run("Missing", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodGet, "/payments/nonsense", nil, token)
		validateError(t, http.StatusNotFound, recorder, "Transaction not found")
	})
}

func TestPaymentsRefund(t *testing.T) {
	url := "/payments/" + testData.firstTransaction.ID + "/refund"
	t.Run("MismatchedCurrency", func(t *testing.T) {
		w := runPaymentRefund(t, url, &PaymentParams{
			Amount:   1,
			Currency: "monopoly-money",
		})
		validateError(t, http.StatusBadRequest, w, "Currencies do not match")
	})
	t.Run("AmountTooHighOrLow", func(t *testing.T) {
		w := runPaymentRefund(t, url, &PaymentParams{
			Amount:   1000,
			Currency: "usd",
		})
		validateError(t, http.StatusBadRequest, w, "must be between 0 and the total amount")
	})
	t.Run("UnknownPayment", func(t *testing.T) {
		w := runPaymentRefund(t, "/payments/nothing/refund", &stripePaymentParams{
			Amount:      1,
			Currency:    testData.firstTransaction.Currency,
			StripeToken: "123",
		})
		validateError(t, http.StatusNotFound, w)
	})
	t.Run("Unpaid", func(t *testing.T) {
		test := NewRouteTest(t)
		testData.firstTransaction.Status = models.PendingState
		test.DB.Save(testData.firstTransaction)

		params := &stripePaymentParams{
			Amount:      1,
			Currency:    testData.firstTransaction.Currency,
			StripeToken: "123",
		}
		body, err := json.Marshal(params)
		require.NoError(t, err)
		token := testAdminToken("magical-unicorn", "")
		w := test.TestEndpoint(http.MethodPost, url, bytes.NewBuffer(body), token)
		validateError(t, http.StatusBadRequest, w, "hasn't been paid")
	})
	t.Run("Success", func(t *testing.T) {
		test := NewRouteTest(t)
		// unused, but needed to pass safety check
		test.Config.Payment.Stripe.Enabled = true
		test.Config.Payment.Stripe.SecretKey = "secret"

		provider := &memProvider{name: payments.StripeProvider}
		ctx, err := withTenantConfig(context.Background(), test.Config)
		require.NoError(t, err)
		ctx = gcontext.WithPaymentProviders(ctx, map[string]payments.Provider{payments.StripeProvider: provider})

		params := &stripePaymentParams{
			Amount:      1,
			Currency:    testData.firstTransaction.Currency,
			StripeToken: "123",
		}
		body, err := json.Marshal(params)
		require.NoError(t, err)
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", url, bytes.NewBuffer(body))
		err = signHTTPRequest(r, testAdminToken("magical-unicorn", ""), test.Config.JWT.Secret)
		require.NoError(t, err)

		NewAPIWithVersion(ctx, test.GlobalConfig, test.DB, defaultVersion).handler.ServeHTTP(w, r)

		rsp := new(models.Transaction)
		extractPayload(t, http.StatusOK, w, rsp)

		stored := &models.Transaction{ID: rsp.ID}
		test.DB.First(stored)

		for _, payment := range []*models.Transaction{stored, rsp} {
			assert.NotEmpty(t, payment.ID)
			assert.Equal(t, testData.testUser.ID, payment.UserID)
			assert.EqualValues(t, 1, payment.Amount)
			assert.Equal(t, "usd", payment.Currency)
			assert.Empty(t, payment.FailureCode)
			assert.Empty(t, payment.FailureDescription)
			assert.Equal(t, models.RefundTransactionType, payment.Type)
			assert.Equal(t, models.PaidState, payment.Status)
		}
	})

	t.Run("PayPal", func(t *testing.T) {
		var loginCount, refundCount int
		refundID := "4CF18861HF410323U"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/v1/oauth2/token":
				w.Header().Add("Content-Type", "application/json")
				fmt.Fprint(w, `{"access_token":"EEwJ6tF9x5WCIZDYzyZGaz6Khbw7raYRIBV_WxVvgmsG","expires_in":100000}`)
				loginCount++
			case "/v1/payments/sale/" + testData.secondTransaction.ProcessorID + "/refund":
				w.Header().Add("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"`+refundID+`"}`)
				refundCount++
			default:
				w.WriteHeader(500)
				t.Fatalf("unknown PayPal API call to %s", r.URL.Path)
			}
		}))
		defer server.Close()

		test := NewRouteTest(t)
		test.Config.Payment.PayPal.Enabled = true
		test.Config.Payment.PayPal.ClientID = "clientid"
		test.Config.Payment.PayPal.Secret = "secret"
		test.Config.Payment.PayPal.Env = server.URL

		params := &paypalPaymentParams{
			Amount:       1,
			Currency:     testData.secondTransaction.Currency,
			PaypalID:     "123",
			PaypalUserID: "456",
		}

		body, err := json.Marshal(params)
		require.NoError(t, err)

		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodPost, "/payments/"+testData.secondTransaction.ID+"/refund", bytes.NewBuffer(body), token)

		rsp := models.Transaction{}
		extractPayload(t, http.StatusOK, recorder, &rsp)
		assert.Equal(t, refundID, rsp.ProcessorID)
		assert.Equal(t, 1, loginCount, "too many login calls")
		assert.Equal(t, 1, refundCount, "too many refund calls")
	})
}

func runPaymentRefund(t *testing.T, url string, params interface{}) *httptest.ResponseRecorder {
	test := NewRouteTest(t)
	body, err := json.Marshal(params)
	require.NoError(t, err)
	token := testAdminToken("magical-unicorn", "")
	return test.TestEndpoint(http.MethodPost, url, bytes.NewBuffer(body), token)
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

func validateAllTransactions(t *testing.T, testData *TestData, trans []models.Transaction) {
	assert.Equal(t, 2, len(trans))
	for _, tran := range trans {
		switch tran.ID {
		case testData.secondTransaction.ID:
			validateTransaction(t, testData.secondTransaction, &tran)
		case testData.firstTransaction.ID:
			validateTransaction(t, testData.firstTransaction, &tran)
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
