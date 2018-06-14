package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"errors"

	"fmt"

	"strings"

	"github.com/netlify/gocommerce/conf"
	gcontext "github.com/netlify/gocommerce/context"
	"github.com/netlify/gocommerce/models"
	"github.com/netlify/gocommerce/payments"
	stripe "github.com/stripe/stripe-go"
)

// ------------------------------------------------------------------------------------------------
// List by ORDER
// ------------------------------------------------------------------------------------------------

func TestOrderPaymentsList(t *testing.T) {
	t.Run("AsOwner", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testToken(test.Data.testUser.ID, "")
		recorder := test.TestEndpoint(http.MethodGet, test.Data.urlForFirstOrder+"/payments", nil, token)

		// we should have gotten back a list of transactions
		trans := []models.Transaction{}
		extractPayload(t, http.StatusOK, recorder, &trans)
		assert.Len(t, trans, 1)
		validateTransaction(t, test.Data.firstTransaction, &trans[0])
	})

	t.Run("AsAdmin", func(t *testing.T) {
		test := NewRouteTest(t)
		anotherTransaction := models.NewTransaction(test.Data.firstOrder)
		test.DB.Create(anotherTransaction)
		defer test.DB.Unscoped().Delete(anotherTransaction)

		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodGet, test.Data.urlForFirstOrder+"/payments", nil, token)

		trans := []models.Transaction{}
		extractPayload(t, http.StatusOK, recorder, &trans)
		assert.Len(t, trans, 2)
		for _, tran := range trans {
			switch tran.ID {
			case anotherTransaction.ID:
				validateTransaction(t, anotherTransaction, &tran)
			case test.Data.firstTransaction.ID:
				validateTransaction(t, test.Data.firstTransaction, &tran)
			default:
				assert.Fail(t, "Unknown transaction: "+tran.ID)
			}
		}
	})

	t.Run("Anonymous", func(t *testing.T) {
		test := NewRouteTest(t)
		recorder := test.TestEndpoint(http.MethodGet, test.Data.urlForFirstOrder+"/payments", nil, nil)
		validateError(t, http.StatusUnauthorized, recorder)
	})
}

// ------------------------------------------------------------------------------------------------
// List by USER
// ------------------------------------------------------------------------------------------------

func TestUserPaymentsList(t *testing.T) {

	t.Run("AsUser", func(t *testing.T) {
		test := NewRouteTest(t)
		url := "/users/" + test.Data.testUser.ID + "/payments"
		token := testToken(test.Data.testUser.ID, "")
		recorder := test.TestEndpoint(http.MethodGet, url, nil, token)

		actual := []models.Transaction{}
		extractPayload(t, http.StatusOK, recorder, &actual)
		validateAllTransactions(t, test.Data, actual)
	})

	t.Run("AsAdmin", func(t *testing.T) {
		test := NewRouteTest(t)
		url := "/users/" + test.Data.testUser.ID + "/payments"
		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodGet, url, nil, token)

		actual := []models.Transaction{}
		extractPayload(t, http.StatusOK, recorder, &actual)
		validateAllTransactions(t, test.Data, actual)
	})

	t.Run("Anonymous", func(t *testing.T) {
		test := NewRouteTest(t)
		url := "/users/" + test.Data.testUser.ID + "/payments"
		recorder := test.TestEndpoint(http.MethodGet, url, nil, nil)
		validateError(t, http.StatusUnauthorized, recorder)
	})

	t.Run("AsStranger", func(t *testing.T) {
		test := NewRouteTest(t)
		url := "/users/" + test.Data.testUser.ID + "/payments"
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
		validateTransaction(t, test.Data.firstTransaction, &trans[0])
	})

	t.Run("NoParams", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodGet, url, nil, token)

		trans := []models.Transaction{}
		extractPayload(t, http.StatusOK, recorder, &trans)
		validateAllTransactions(t, test.Data, trans)
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
		recorder := test.TestEndpoint(http.MethodGet, "/payments/"+test.Data.firstTransaction.ID, nil, token)

		trans := new(models.Transaction)
		extractPayload(t, http.StatusOK, recorder, trans)
		validateTransaction(t, test.Data.firstTransaction, trans)
	})

	t.Run("Missing", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodGet, "/payments/nonsense", nil, token)
		validateError(t, http.StatusNotFound, recorder, "Transaction not found")
	})
}

func TestPaymentsRefund(t *testing.T) {
	t.Run("MismatchedCurrency", func(t *testing.T) {
		test := NewRouteTest(t)
		url := "/payments/" + test.Data.firstTransaction.ID + "/refund"
		w := runPaymentRefund(test, url, &PaymentParams{
			Amount:   1,
			Currency: "monopoly-money",
		})
		validateError(t, http.StatusBadRequest, w, "Currencies do not match")
	})
	t.Run("AmountTooHighOrLow", func(t *testing.T) {
		test := NewRouteTest(t)
		url := "/payments/" + test.Data.firstTransaction.ID + "/refund"
		w := runPaymentRefund(test, url, &PaymentParams{
			Amount:   1000,
			Currency: "USD",
		})
		validateError(t, http.StatusBadRequest, w, "must be between 0 and the total amount")
	})
	t.Run("UnknownPayment", func(t *testing.T) {
		test := NewRouteTest(t)
		w := runPaymentRefund(test, "/payments/nothing/refund", &stripePaymentParams{
			Amount:      1,
			Currency:    test.Data.firstTransaction.Currency,
			StripeToken: "123",
		})
		validateError(t, http.StatusNotFound, w)
	})
	t.Run("Unpaid", func(t *testing.T) {
		test := NewRouteTest(t)
		url := "/payments/" + test.Data.firstTransaction.ID + "/refund"
		test.Data.firstTransaction.Status = models.PendingState
		test.DB.Save(test.Data.firstTransaction)

		params := &stripePaymentParams{
			Amount:      1,
			Currency:    test.Data.firstTransaction.Currency,
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
		url := "/payments/" + test.Data.firstTransaction.ID + "/refund"
		// unused, but needed to pass safety check
		test.Config.Payment.Stripe.Enabled = true
		test.Config.Payment.Stripe.SecretKey = "secret"

		globalConfig := new(conf.GlobalConfiguration)
		provider := &memProvider{name: payments.StripeProvider}
		ctx, err := WithInstanceConfig(context.Background(), globalConfig.SMTP, test.Config, "")
		require.NoError(t, err)
		ctx = gcontext.WithPaymentProviders(ctx, map[string]payments.Provider{payments.StripeProvider: provider})

		params := &stripePaymentParams{
			Amount:      1,
			Currency:    test.Data.firstTransaction.Currency,
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
			assert.Equal(t, test.Data.testUser.ID, payment.UserID)
			assert.EqualValues(t, 1, payment.Amount)
			assert.Equal(t, "USD", payment.Currency)
			assert.Empty(t, payment.FailureCode)
			assert.Empty(t, payment.FailureDescription)
			assert.Equal(t, models.RefundTransactionType, payment.Type)
			assert.Equal(t, models.PaidState, payment.Status)
		}
	})

	t.Run("PayPal", func(t *testing.T) {
		test := NewRouteTest(t)
		var loginCount, refundCount int
		refundID := "4CF18861HF410323U"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/v1/oauth2/token":
				w.Header().Add("Content-Type", "application/json")
				fmt.Fprint(w, `{"access_token":"EEwJ6tF9x5WCIZDYzyZGaz6Khbw7raYRIBV_WxVvgmsG","expires_in":100000}`)
				loginCount++
			case "/v1/payments/sale/" + test.Data.secondTransaction.ProcessorID + "/refund":
				w.Header().Add("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"`+refundID+`"}`)
				refundCount++
			default:
				w.WriteHeader(500)
				t.Fatalf("unknown PayPal API call to %s", r.URL.Path)
			}
		}))
		defer server.Close()

		test.Config.Payment.PayPal.Enabled = true
		test.Config.Payment.PayPal.ClientID = "clientid"
		test.Config.Payment.PayPal.Secret = "secret"
		test.Config.Payment.PayPal.Env = server.URL

		params := &paypalPaymentParams{
			Amount:       1,
			Currency:     test.Data.secondTransaction.Currency,
			PaypalID:     "123",
			PaypalUserID: "456",
		}

		body, err := json.Marshal(params)
		require.NoError(t, err)

		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodPost, "/payments/"+test.Data.secondTransaction.ID+"/refund", bytes.NewBuffer(body), token)

		rsp := models.Transaction{}
		extractPayload(t, http.StatusOK, recorder, &rsp)
		assert.Equal(t, refundID, rsp.ProcessorID)
		assert.Equal(t, 1, loginCount, "too many login calls")
		assert.Equal(t, 1, refundCount, "too many refund calls")
	})
}

func runPaymentRefund(test *RouteTest, url string, params interface{}) *httptest.ResponseRecorder {
	body, err := json.Marshal(params)
	require.NoError(test.T, err)
	token := testAdminToken("magical-unicorn", "")
	return test.TestEndpoint(http.MethodPost, url, bytes.NewBuffer(body), token)
}

func TestPaymentCreate(t *testing.T) {
	t.Run("PayPal", func(t *testing.T) {
		t.Run("Simple", func(t *testing.T) {
			test := NewRouteTest(t)
			test.Data.secondOrder.PaymentState = models.PendingState
			rsp := test.DB.Save(test.Data.secondOrder)
			require.NoError(t, rsp.Error, "Failed to update order")

			var loginCount, paymentCount int
			paymentID := "4CF18861HF410323V"
			amtString := fmt.Sprintf("%.2f", float64(test.Data.secondOrder.Total)/100)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1/oauth2/token":
					w.Header().Add("Content-Type", "application/json")
					fmt.Fprint(w, `{"access_token":"EEwJ6tF9x5WCIZDYzyZGaz6Khbw7raYRIBV_WxVvgmsG","expires_in":100000}`)
					loginCount++
				case "/v1/payments/payment/" + paymentID:
					w.Header().Add("Content-Type", "application/json")
					fmt.Fprint(w, `{"id":"`+paymentID+`","transactions":[{"amount":{"total":"`+amtString+`","currency":"`+test.Data.secondOrder.Currency+`"}}]}`)
					paymentCount++
				case "/v1/payments/payment/" + paymentID + "/execute":
					w.Header().Add("Content-Type", "application/json")
					fmt.Fprint(w, `{"id":"`+paymentID+`"}`)
					paymentCount++
				default:
					w.WriteHeader(500)
					t.Fatalf("unknown PayPal API call to %s", r.URL.Path)
				}
			}))
			defer server.Close()
			test.Config.Payment.PayPal.Enabled = true
			test.Config.Payment.PayPal.ClientID = "clientid"
			test.Config.Payment.PayPal.Secret = "secret"
			test.Config.Payment.PayPal.Env = server.URL

			params := &paypalPaymentParams{
				Amount:       test.Data.secondOrder.Total,
				Currency:     test.Data.secondOrder.Currency,
				PaypalID:     paymentID,
				PaypalUserID: "456",
				Provider:     payments.PayPalProvider,
			}

			body, err := json.Marshal(params)
			require.NoError(t, err)

			recorder := test.TestEndpoint(http.MethodPost, "/orders/second-order/payments", bytes.NewBuffer(body), test.Data.testUserToken)

			trans := models.Transaction{}
			extractPayload(t, http.StatusOK, recorder, &trans)
			assert.Equal(t, paymentID, trans.ProcessorID)
			assert.Equal(t, models.PaidState, trans.Status)
			assert.Equal(t, 1, loginCount, "too many login calls")
			assert.Equal(t, 2, paymentCount, "too many payment calls")
		})
	})
	t.Run("Stripe", func(t *testing.T) {
		callCount := 0
		stripe.SetBackend(stripe.APIBackend, NewTrackingStripeBackend(func(method, path, key string, body *stripe.RequestValues, params *stripe.Params) {
			switch path {
			case "/charges":
				callCount++
			default:
				t.Fatalf("unknown Stripe API call to %s", path)
			}
		}))
		defer stripe.SetBackend(stripe.APIBackend, nil)

		test := NewRouteTest(t)
		test.Data.firstOrder.PaymentState = models.PendingState
		rsp := test.DB.Save(test.Data.firstOrder)
		require.NoError(t, rsp.Error, "Failed to update order")

		params := &stripePaymentParams{
			Amount:      test.Data.firstOrder.Total,
			Currency:    test.Data.firstOrder.Currency,
			StripeToken: "123456",
			Provider:    payments.StripeProvider,
		}

		body, err := json.Marshal(params)
		require.NoError(t, err)

		recorder := test.TestEndpoint(http.MethodPost, "/orders/first-order/payments", bytes.NewBuffer(body), test.Data.testUserToken)

		trans := models.Transaction{}
		extractPayload(t, http.StatusOK, recorder, &trans)
		assert.Equal(t, models.PaidState, trans.Status)
		assert.Equal(t, 1, callCount)
	})
}

func TestPaymentPreauthorize(t *testing.T) {
	t.Run("PayPal", func(t *testing.T) {
		testURL := "/paypal"
		var createData *paypalPaymentCreateParams
		var loginCount, paymentCount int
		paymentID := "4CF18861HF410323V"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/v1/oauth2/token":
				w.Header().Add("Content-Type", "application/json")
				fmt.Fprint(w, `{"access_token":"EEwJ6tF9x5WCIZDYzyZGaz6Khbw7raYRIBV_WxVvgmsG","expires_in":100000}`)
				loginCount++
			case "/v1/payment-experience/web-profiles":
				w.Header().Add("Content-Type", "application/json")
				if r.Method == http.MethodGet {
					fmt.Fprint(w, `[{"id":"expid","name":"gocommerce"}]`)
				} else {
					fmt.Fprint(w, `{"id":"expid","name":"gocommerce-asdf"}`)
				}
			case "/v1/payments/payment":
				w.Header().Add("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"`+paymentID+`"}`)
				paymentCount++
				require.NoError(t, json.NewDecoder(r.Body).Decode(createData))
			default:
				w.WriteHeader(500)
				t.Fatalf("unknown PayPal API call to %s", r.URL.Path)
			}
		}))
		defer server.Close()

		t.Run("Form", func(t *testing.T) {
			loginCount = 0
			paymentCount = 0
			createData = new(paypalPaymentCreateParams)
			test := NewRouteTest(t)
			test.Config.Payment.PayPal.Enabled = true
			test.Config.Payment.PayPal.ClientID = "clientid"
			test.Config.Payment.PayPal.Secret = "secret"
			test.Config.Payment.PayPal.Env = server.URL

			form := url.Values{}
			form.Add("provider", payments.PayPalProvider)
			form.Add("amount", "1000")
			form.Add("currency", "USD")
			form.Add("description", "test")

			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, baseURL+testURL, strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")

			globalConfig := new(conf.GlobalConfiguration)
			ctx, err := WithInstanceConfig(context.Background(), globalConfig.SMTP, test.Config, "")
			require.NoError(t, err)
			NewAPIWithVersion(ctx, test.GlobalConfig, test.DB, "").handler.ServeHTTP(recorder, req)

			rsp := payments.PreauthorizationResult{}
			extractPayload(t, http.StatusOK, recorder, &rsp)
			assert.Equal(t, paymentID, rsp.ID)
			assert.Equal(t, 1, loginCount, "too many login calls")
			assert.Equal(t, 1, paymentCount, "too many payment calls")

			require.Len(t, createData.Transactions, 1)
			assert.Equal(t, "sale", createData.Intent)
			assert.Equal(t, "10.00", createData.Transactions[0].Amount.Total)
			assert.Equal(t, "USD", createData.Transactions[0].Amount.Currency)
			assert.Equal(t, "test", createData.Transactions[0].Description)
		})
		t.Run("JSON", func(t *testing.T) {
			loginCount = 0
			paymentCount = 0
			createData = new(paypalPaymentCreateParams)
			test := NewRouteTest(t)
			test.Config.Payment.PayPal.Enabled = true
			test.Config.Payment.PayPal.ClientID = "clientid"
			test.Config.Payment.PayPal.Secret = "secret"
			test.Config.Payment.PayPal.Env = server.URL

			params := paypalPreauthorizeParams{
				Amount:      1000,
				Currency:    "USD",
				Description: "test",
				Provider:    payments.PayPalProvider,
			}

			body, err := json.Marshal(params)
			require.NoError(t, err)

			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, baseURL+testURL, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			globalConfig := new(conf.GlobalConfiguration)
			ctx, err := WithInstanceConfig(context.Background(), globalConfig.SMTP, test.Config, "")
			require.NoError(t, err)
			NewAPIWithVersion(ctx, test.GlobalConfig, test.DB, "").handler.ServeHTTP(recorder, req)

			rsp := payments.PreauthorizationResult{}
			extractPayload(t, http.StatusOK, recorder, &rsp)
			assert.Equal(t, paymentID, rsp.ID)
			assert.Equal(t, 1, loginCount, "too many login calls")
			assert.Equal(t, 1, paymentCount, "too many payment calls")

			require.Len(t, createData.Transactions, 1)
			assert.Equal(t, "sale", createData.Intent)
			assert.Equal(t, "10.00", createData.Transactions[0].Amount.Total)
			assert.Equal(t, "USD", createData.Transactions[0].Amount.Currency)
			assert.Equal(t, "test", createData.Transactions[0].Description)
		})
	})
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
	Amount      uint64 `json:"amount"`
	Currency    string `json:"currency"`
	StripeToken string `json:"stripe_token"`
	Provider    string `json:"provider"`
}

type paypalPaymentParams struct {
	Amount       uint64 `json:"amount"`
	Currency     string `json:"currency"`
	PaypalID     string `json:"paypal_payment_id"`
	PaypalUserID string `json:"paypal_user_id"`
	Provider     string `json:"provider"`
}

type paypalPreauthorizeParams struct {
	Amount      uint64 `json:"amount"`
	Currency    string `json:"currency"`
	Description string `json:"description"`
	Provider    string `json:"provider"`
}

type paypalAmount struct {
	Total    string `json:"total"`
	Currency string `json:"currency"`
}

type paypalTransaction struct {
	Amount      paypalAmount `json:"amount"`
	Description string       `json:"description"`
}

type paypalPaymentCreateParams struct {
	Intent       string              `json:"intent"`
	Transactions []paypalTransaction `json:"transactions"`
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

type stripeCallFunc func(method, path, key string, body *stripe.RequestValues, params *stripe.Params)

func NewTrackingStripeBackend(fn stripeCallFunc) stripe.Backend {
	return &trackingStripeBackend{fn}
}

type trackingStripeBackend struct {
	trackingFunc stripeCallFunc
}

func (t trackingStripeBackend) Call(method, path, key string, body *stripe.RequestValues, params *stripe.Params, v interface{}) error {
	t.trackingFunc(method, path, key, body, params)
	return nil
}

func (t trackingStripeBackend) CallMultipart(method, path, key, boundary string, body io.Reader, params *stripe.Params, v interface{}) error {
	return nil
}
