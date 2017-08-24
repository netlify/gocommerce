package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"

	"github.com/netlify/gocommerce/claims"
	"github.com/netlify/gocommerce/models"
	"github.com/stretchr/testify/require"
)

// ------------------------------------------------------------------------------------------------
// CREATE
// ------------------------------------------------------------------------------------------------

func TestOrderCreate(t *testing.T) {
	server := startTestSite()
	defer server.Close()

	t.Run("Simple", func(t *testing.T) {
		test := NewRouteTest(t)
		test.Config.SiteURL = server.URL
		body := strings.NewReader(`{
			"email": "info@example.com",
			"shipping_address": {
				"name": "Test User",
				"address1": "610 22nd Street",
				"city": "San Francisco", "state": "CA", "country": "USA", "zip": "94107"
			},
			"line_items": [{"path": "/simple-product", "quantity": 1, "meta": {"attendees": [{"name": "Matt", "email": "matt@example.com"}]}}]
		}`)
		token := test.Data.testUserToken
		recorder := test.TestEndpoint(http.MethodPost, "/orders", body, token)

		order := &models.Order{}
		extractPayload(t, http.StatusCreated, recorder, order)
		var total uint64 = 999
		assert.Equal(t, "info@example.com", order.Email, "Total should be info@example.com, was %v", order.Email)
		assert.Equal(t, total, order.Total, fmt.Sprintf("Total should be 999, was %v", order.Total))
		assert.Len(t, order.LineItems, 1)
		meta := order.LineItems[0].MetaData
		require.NotNil(t, meta, "Expected meta data for line item")
		_, ok := meta["attendees"]
		require.True(t, ok, "Line item should have attendees")

		stored := &models.Address{ID: order.BillingAddressID}
		require.NoError(t, test.DB.First(stored).Error)
		assert.Equal(t, stored.UserID, order.UserID)
	})

	t.Run("NameBackwardsCompatible", func(t *testing.T) {
		test := NewRouteTest(t)
		test.Config.SiteURL = server.URL
		body := strings.NewReader(`{
			"email": "info@example.com",
			"shipping_address": {
				"first_name": "Test", "last_name": "User",
				"address1": "610 22nd Street",
				"city": "San Francisco", "state": "CA", "country": "USA", "zip": "94107"
			},
			"line_items": [{"path": "/simple-product", "quantity": 1, "meta": {"attendees": [{"name": "Matt", "email": "matt@example.com"}]}}]
		}`)
		token := test.Data.testUserToken
		recorder := test.TestEndpoint(http.MethodPost, "/orders", body, token)

		order := &models.Order{}
		extractPayload(t, http.StatusCreated, recorder, order)
		assert.Equal(t, "Test User", order.ShippingAddress.Name)
	})

	t.Run("WithTaxes", func(t *testing.T) {
		test := NewRouteTest(t)
		test.Config.SiteURL = server.URL
		body := strings.NewReader(`{
			"email": "info@example.com",
			"shipping_address": {
				"name": "Test User",
				"address1": "Branengebranen",
				"city": "Berlin", "country": "Germany", "zip": "94107"
			},
			"line_items": [{"path": "/simple-product", "quantity": 1}]
		}`)
		token := test.Data.testUserToken
		recorder := test.TestEndpoint(http.MethodPost, "/orders", body, token)

		order := &models.Order{}
		extractPayload(t, http.StatusCreated, recorder, order)
		var total uint64 = 1069
		var taxes uint64 = 70
		assert.Equal(t, "info@example.com", order.Email, "Total should be info@example.com, was %v", order.Email)
		assert.Equal(t, "Germany", order.ShippingAddress.Country)
		assert.Equal(t, "Germany", order.BillingAddress.Country)
		assert.Equal(t, total, order.Total, fmt.Sprintf("Total should be 1069, was %v", order.Total))
		assert.Equal(t, taxes, order.Taxes, fmt.Sprintf("Total should be 70, was %v", order.Total))
	})

	t.Run("BundleWithTaxes", func(t *testing.T) {
		test := NewRouteTest(t)
		test.Config.SiteURL = server.URL
		body := strings.NewReader(`{
			"email": "info@example.com",
			"shipping_address": {
				"name": "Test User",
				"address1": "Branengebranen",
				"city": "Berlin", "country": "Germany", "zip": "94107"
			},
			"line_items": [{"path": "/bundle-product", "quantity": 1}]
		}`)
		token := test.Data.testUserToken
		recorder := test.TestEndpoint(http.MethodPost, "/orders", body, token)

		order := &models.Order{}
		extractPayload(t, http.StatusCreated, recorder, order)
		var total uint64 = 1105
		var taxes uint64 = 106
		assert.Equal(t, "info@example.com", order.Email, "Total should be info@example.com, was %v", order.Email)
		assert.Equal(t, "Germany", order.ShippingAddress.Country)
		assert.Equal(t, "Germany", order.BillingAddress.Country)
		assert.Equal(t, total, order.Total, fmt.Sprintf("Total should be 1105, was %v", order.Total))
		assert.Equal(t, taxes, order.Taxes, fmt.Sprintf("Total should be 106, was %v", order.Total))
	})
}

// ------------------------------------------------------------------------------------------------
// LIST
// ------------------------------------------------------------------------------------------------

func TestOrdersList(t *testing.T) {
	t.Run("AsTheUser", func(t *testing.T) {
		test := NewRouteTest(t)
		token := test.Data.testUserToken
		recorder := test.TestEndpoint(http.MethodGet, "/orders", nil, token)

		orders := []models.Order{}
		extractPayload(t, http.StatusOK, recorder, &orders)
		assert.Len(t, orders, 2)
		validateAllOrders(t, orders, test.Data)
	})
	t.Run("AsStranger", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testToken("stranger", "stranger-danger@wayneindustries.com")
		recorder := test.TestEndpoint(http.MethodGet, "/orders", nil, token)

		orders := []models.Order{}
		extractPayload(t, http.StatusOK, recorder, &orders)
		assert.Len(t, orders, 0)
	})
	t.Run("AsExpiredToken", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testExpiredToken("stranger", "stranger-danger@wayneindustries.com")
		recorder := test.TestEndpoint(http.MethodGet, "/orders", nil, token)
		validateError(t, http.StatusUnauthorized, recorder)
	})
	t.Run("Filter", func(t *testing.T) {
		t.Run("EmailFilterAsTheUser", func(t *testing.T) {
			test := NewRouteTest(t)
			token := test.Data.testUserToken
			recorder := test.TestEndpoint(http.MethodGet, "/orders?email=bruce", nil, token)

			orders := []models.Order{}
			extractPayload(t, http.StatusOK, recorder, &orders)
			assert.Len(t, orders, 2)
		})
		t.Run("EmailFilterAsTheUserEmptyResponse", func(t *testing.T) {
			test := NewRouteTest(t)
			token := test.Data.testUserToken
			recorder := test.TestEndpoint(http.MethodGet, "/orders?email=gmail.com", nil, token)

			orders := []models.Order{}
			extractPayload(t, http.StatusOK, recorder, &orders)
			assert.Len(t, orders, 0)
		})
		t.Run("ItemFilterAsTheUser", func(t *testing.T) {
			test := NewRouteTest(t)
			token := test.Data.testUserToken
			recorder := test.TestEndpoint(http.MethodGet, "/orders?items=batwing", nil, token)

			orders := []models.Order{}
			extractPayload(t, http.StatusOK, recorder, &orders)
			assert.Len(t, orders, 1)
		})
		t.Run("BillingNameFilterAsTheUser", func(t *testing.T) {
			test := NewRouteTest(t)
			token := test.Data.testUserToken
			recorder := test.TestEndpoint(http.MethodGet, "/orders?billing_name=whatname", nil, token)

			orders := []models.Order{}
			extractPayload(t, http.StatusOK, recorder, &orders)
			assert.Len(t, orders, 0)
		})
		t.Run("ShippingNameFilterAsTheUser", func(t *testing.T) {
			test := NewRouteTest(t)
			token := test.Data.testUserToken
			recorder := test.TestEndpoint(http.MethodGet, "/orders?shipping_name=whatname", nil, token)

			orders := []models.Order{}
			extractPayload(t, http.StatusOK, recorder, &orders)
			assert.Len(t, orders, 0)
		})
		t.Run("ItemTypeFilterAsTheUser", func(t *testing.T) {
			test := NewRouteTest(t)
			token := test.Data.testUserToken
			recorder := test.TestEndpoint(http.MethodGet, "/orders?item_type=plane", nil, token)

			orders := []models.Order{}
			extractPayload(t, http.StatusOK, recorder, &orders)
			assert.Len(t, orders, 1)
		})
		t.Run("CouponCodeFilterAsTheUser", func(t *testing.T) {
			test := NewRouteTest(t)
			token := test.Data.testUserToken
			recorder := test.TestEndpoint(http.MethodGet, "/orders?coupon_code=zerodiscount", nil, token)

			orders := []models.Order{}
			extractPayload(t, http.StatusOK, recorder, &orders)
			assert.Len(t, orders, 1)
		})
	})
}

func TestUserOrdersList(t *testing.T) {
	t.Run("AllOrders", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testAdminToken("admin-yo", "admin@wayneindustries.com")
		recorder := test.TestEndpoint(http.MethodGet, "/users/all/orders", nil, token)

		orders := []models.Order{}
		extractPayload(t, http.StatusOK, recorder, &orders)
		assert.Len(t, orders, 2)
		validateAllOrders(t, orders, test.Data)
	})
	t.Run("NotWithAdminRights", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testToken("stranger", "stranger-danger@wayneindustries.com")
		recorder := test.TestEndpoint(http.MethodGet, "/users/all/orders", nil, token)
		validateError(t, http.StatusUnauthorized, recorder)
	})
	t.Run("Anonymous", func(t *testing.T) {
		test := NewRouteTest(t)
		recorder := test.TestEndpoint(http.MethodGet, "/users/all/orders", nil, nil)
		validateError(t, http.StatusUnauthorized, recorder)
	})
}

// -------------------------------------------------------------------------------------------------------------------
// VIEW
// -------------------------------------------------------------------------------------------------------------------

func TestOrderView(t *testing.T) {
	t.Run("AsTheUser", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testToken(test.Data.testUser.ID, "marp@wayneindustries.com")
		recorder := test.TestEndpoint(http.MethodGet, test.Data.urlForFirstOrder, nil, token)

		order := new(models.Order)
		extractPayload(t, http.StatusOK, recorder, order)
		validateOrder(t, test.Data.firstOrder, order)
		validateAddress(t, test.Data.firstOrder.BillingAddress, order.BillingAddress)
		validateAddress(t, test.Data.firstOrder.ShippingAddress, order.ShippingAddress)
	})
	t.Run("AsAnAdmin", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testAdminToken("admin-yo", "admin@wayneindustries.com")
		recorder := test.TestEndpoint(http.MethodGet, test.Data.urlForFirstOrder, nil, token)

		order := new(models.Order)
		extractPayload(t, http.StatusOK, recorder, order)
		validateOrder(t, test.Data.firstOrder, order)
		validateAddress(t, test.Data.firstOrder.BillingAddress, order.BillingAddress)
		validateAddress(t, test.Data.firstOrder.ShippingAddress, order.ShippingAddress)
	})
	t.Run("AsAStranger", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testToken("stranger", "stranger-danger@wayneindustries.com")
		recorder := test.TestEndpoint(http.MethodGet, test.Data.urlForFirstOrder, nil, token)
		validateError(t, http.StatusUnauthorized, recorder)
	})
	t.Run("MissingOrder", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testToken("stranger", "stranger-danger@wayneindustries.com")
		recorder := test.TestEndpoint(http.MethodGet, "/orders/does-not-exist", nil, token)
		validateError(t, http.StatusNotFound, recorder)
	})
	t.Run("Anonymous", func(t *testing.T) {
		test := NewRouteTest(t)
		test.Data.firstOrder.User = nil
		test.Data.firstOrder.UserID = ""
		rsp := test.DB.Save(test.Data.firstOrder)
		require.NoError(t, rsp.Error, "Failed to update order")
		recorder := test.TestEndpoint(http.MethodGet, test.Data.urlForFirstOrder, nil, nil)

		order := new(models.Order)
		extractPayload(t, http.StatusOK, recorder, order)
		validateOrder(t, test.Data.firstOrder, order)
		validateAddress(t, test.Data.firstOrder.BillingAddress, order.BillingAddress)
		validateAddress(t, test.Data.firstOrder.ShippingAddress, order.ShippingAddress)
	})
}

// --------------------------------------------------------------------------------------------------------------------
// Create ~ email logic
// --------------------------------------------------------------------------------------------------------------------
func TestOrderSetUserIDLogic(t *testing.T) {
	t.Run("AnonymousUser", func(t *testing.T) {
		simpleOrder := models.NewOrder("", "session", "params@email.com", "USD")
		require.NoError(t, setOrderEmail(nil, simpleOrder, nil, testLogger))
		assert.Equal(t, "params@email.com", simpleOrder.Email)
	})
	t.Run("AnonymousUserNoEmail", func(t *testing.T) {
		simpleOrder := models.NewOrder("", "session", "", "USD")
		err := setOrderEmail(nil, simpleOrder, nil, testLogger)
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, err.Code)
	})
	t.Run("NewUserNoEmailOnRequest", func(t *testing.T) {
		validateNewUserEmail(
			t,
			models.NewOrder("", "session", "", "USD"),
			testToken("alfred", "alfred@wayne.com").Claims.(*claims.JWTClaims),
			"alfred@wayne.com",
			"alfred@wayne.com",
		)
	})
	t.Run("NewUserNoEmailOnClaim", func(t *testing.T) {
		validateNewUserEmail(
			t,
			models.NewOrder("", "session", "joker@wayne.com", "USD"),
			testToken("alfred", "").Claims.(*claims.JWTClaims),
			"",
			"joker@wayne.com",
		)
	})
	t.Run("NewUserAllTheEmails", func(t *testing.T) {
		validateNewUserEmail(
			t,
			models.NewOrder("", "session", "joker@wayne.com", "USD"),
			testToken("alfred", "alfred@wayne.com").Claims.(*claims.JWTClaims),
			"alfred@wayne.com",
			"joker@wayne.com",
		)
	})
	t.Run("NewUserNoEmails", func(t *testing.T) {
		db, _, _, _ := db(t)
		simpleOrder := models.NewOrder("", "session", "", "USD")
		claims := testToken("alfred", "").Claims.(*claims.JWTClaims)
		err := setOrderEmail(db, simpleOrder, claims, testLogger)
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, err.Code)
	})
	t.Run("KnownUserClaimsOnRequest", func(t *testing.T) {
		db, _, _, testData := db(t)
		validateExistingUserEmail(
			t,
			db,
			models.NewOrder("", "session", "joker@wayne.com", "USD"),
			testToken(testData.testUser.ID, "").Claims.(*claims.JWTClaims),
			"joker@wayne.com",
		)
	})
	t.Run("KnownUserClaimsOnClaim", func(t *testing.T) {
		db, _, _, testData := db(t)
		validateExistingUserEmail(
			t,
			db,
			models.NewOrder("", "session", "", "USD"),
			testToken(testData.testUser.ID, testData.testUser.Email).Claims.(*claims.JWTClaims),
			testData.testUser.Email,
		)
	})
	t.Run("KnownUserAllTheEmail", func(t *testing.T) {
		db, _, _, testData := db(t)
		validateExistingUserEmail(
			t,
			db,
			models.NewOrder("", "session", "joker@wayne.com", "USD"),
			testToken(testData.testUser.ID, testData.testUser.Email).Claims.(*claims.JWTClaims),
			"joker@wayne.com",
		)
	})
	t.Run("KnownUserNoEmail", func(t *testing.T) {
		db, _, _, testData := db(t)
		validateExistingUserEmail(
			t,
			db,
			models.NewOrder("", "session", "", "USD"),
			testToken(testData.testUser.ID, "").Claims.(*claims.JWTClaims),
			testData.testUser.Email,
		)
	})
}

// --------------------------------------------------------------------------------------------------------------------
// UPDATE
// --------------------------------------------------------------------------------------------------------------------

func TestOrderUpdate(t *testing.T) {
	t.Run("FieldsUpdate", func(t *testing.T) {
		test := NewRouteTest(t)
		test.Data.firstOrder.PaymentState = models.PendingState
		rsp := test.DB.Save(test.Data.firstOrder)
		require.NoError(t, rsp.Error, "Failed to update email")

		op := &orderRequestParams{
			Email:    "mrfreeze@dc.com",
			Currency: "monopoly-dollars",
		}
		token := testAdminToken("admin-yo", "admin@wayneindustries.com")
		recorder := runOrderUpdate(test, test.Data.firstOrder, op, token)
		defer test.DB.Save(test.Data.firstOrder)

		assert := assert.New(t)
		rspOrder := new(models.Order)
		extractPayload(t, http.StatusOK, recorder, rspOrder)

		saved := new(models.Order)
		rsp = test.DB.Preload("LineItems").First(saved, "id = ?", test.Data.firstOrder.ID)
		require.False(t, rsp.RecordNotFound())

		assert.Equal("mrfreeze@dc.com", rspOrder.Email)
		assert.Equal("monopoly-dollars", rspOrder.Currency)

		// did it get persisted to the db
		assert.Equal("mrfreeze@dc.com", saved.Email)
		assert.Equal("monopoly-dollars", saved.Currency)
		validateOrder(t, saved, rspOrder)

		// should be the only field that has changed ~ check it
		saved.Email = test.Data.firstOrder.Email
		saved.Currency = test.Data.firstOrder.Currency
		validateOrder(t, test.Data.firstOrder, saved)
	})

	t.Run("ExistingAddress", func(t *testing.T) {
		test := NewRouteTest(t)
		newAddr := getTestAddress()
		newAddr.ID = "new-addr"
		newAddr.UserID = test.Data.firstOrder.UserID
		test.DB.Create(newAddr)
		defer test.DB.Unscoped().Delete(newAddr)

		op := &orderRequestParams{
			BillingAddressID: newAddr.ID,
		}
		token := testAdminToken("admin-yo", "admin@wayneindustries.com")
		recorder := runOrderUpdate(test, test.Data.firstOrder, op, token)
		defer test.DB.Save(test.Data.firstOrder)

		rspOrder := new(models.Order)
		extractPayload(t, http.StatusOK, recorder, rspOrder)

		saved := new(models.Order)
		rsp := test.DB.First(saved, "id = ?", test.Data.firstOrder.ID)
		require.False(t, rsp.RecordNotFound())

		// now we load the addresses
		assert.Equal(t, saved.BillingAddressID, rspOrder.BillingAddressID)

		savedAddr := &models.Address{ID: saved.BillingAddressID}
		rsp = test.DB.First(savedAddr)
		require.False(t, rsp.RecordNotFound())
		defer test.DB.Unscoped().Delete(savedAddr)

		validateAddress(t, *newAddr, *savedAddr)
	})

	t.Run("NewAddress", func(t *testing.T) {
		test := NewRouteTest(t)
		paramsAddress := getTestAddress()
		op := &orderRequestParams{
			// should create a new address associated with the order's user
			ShippingAddress: paramsAddress,
		}
		token := testAdminToken("admin-yo", "admin@wayneindustries.com")
		recorder := runOrderUpdate(test, test.Data.firstOrder, op, token)
		defer test.DB.Save(test.Data.firstOrder)

		rspOrder := new(models.Order)
		extractPayload(t, http.StatusOK, recorder, rspOrder)

		saved := new(models.Order)
		rsp := test.DB.First(saved, "id = ?", test.Data.firstOrder.ID)
		require.False(t, rsp.RecordNotFound())

		// now we load the addresses
		assert.Equal(t, saved.ShippingAddressID, rspOrder.ShippingAddressID)

		savedAddr := &models.Address{ID: saved.ShippingAddressID}
		rsp = test.DB.First(savedAddr)
		require.False(t, rsp.RecordNotFound())
		defer test.DB.Unscoped().Delete(savedAddr)

		validateAddress(t, *paramsAddress, *savedAddr)
	})

	t.Run("NonAdmin", func(t *testing.T) {
		test := NewRouteTest(t)
		op := &orderRequestParams{
			Email:    "mrfreeze@dc.com",
			Currency: "monopoly-dollars",
		}
		token := testToken("villian", "villian@wayneindustries.com")
		recorder := runOrderUpdate(test, test.Data.firstOrder, op, token)
		validateError(t, http.StatusUnauthorized, recorder)
	})

	t.Run("NoCreds", func(t *testing.T) {
		test := NewRouteTest(t)
		op := &orderRequestParams{
			Email:    "mrfreeze@dc.com",
			Currency: "monopoly-dollars",
		}
		recorder := runOrderUpdate(test, test.Data.firstOrder, op, nil)
		validateError(t, http.StatusUnauthorized, recorder)
	})

	t.Run("NewData", func(t *testing.T) {
		test := NewRouteTest(t)
		op := &orderRequestParams{
			MetaData: map[string]interface{}{
				"thing":       float64(1),
				"red":         "fish",
				"other thing": 3.4,
				"exists":      true,
			},
		}
		token := testAdminToken("admin-yo", "admin@wayneindustries.com")
		recorder := runOrderUpdate(test, test.Data.firstOrder, op, token)
		defer test.DB.Save(test.Data.firstOrder)

		order := &models.Order{}
		extractPayload(t, http.StatusOK, recorder, order)
		assert.Equal(t, op.MetaData, order.MetaData, "Order metadata should have been updated")
	})
}

// -------------------------------------------------------------------------------------------------------------------
// CLAIMS
// -------------------------------------------------------------------------------------------------------------------

func TestClaim(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		test := NewRouteTest(t)
		test.Data.firstOrder.Email = "villian@wayneindustries.com"
		test.Data.firstOrder.UserID = ""
		rsp := test.DB.Save(test.Data.firstOrder)
		require.NoError(t, rsp.Error, "Failed to update email")

		token := testToken("villian", "villian@wayneindustries.com")
		recorder := test.TestEndpoint(http.MethodPost, "/claim", nil, token)
		require.Equal(t, http.StatusNoContent, recorder.Code)

		// validate the DB
		dbOrders := []models.Order{}
		rsp = test.DB.Where("email = ?", "villian@wayneindustries.com").Find(&dbOrders)
		require.NoError(t, rsp.Error, "Failed to query DB")

		assert.Len(t, dbOrders, 1)
		assert.Equal(t, "villian@wayneindustries.com", dbOrders[0].Email)
		assert.Equal(t, "villian", dbOrders[0].UserID)

		stored := &models.Address{ID: dbOrders[0].BillingAddressID}
		require.NoError(t, test.DB.First(stored).Error)
		assert.Equal(t, stored.UserID, dbOrders[0].UserID)
	})

	t.Run("NoEmail", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testToken("villian", "")
		recorder := test.TestEndpoint(http.MethodPost, "/claim", nil, token)
		require.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("NoID", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testToken("", "villian@wayneindustries.com")
		recorder := test.TestEndpoint(http.MethodPost, "/claim", nil, token)
		require.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("MultipleTimes", func(t *testing.T) {
		test := NewRouteTest(t)
		test.Data.firstOrder.Email = "villian@wayneindustries.com"
		test.Data.firstOrder.UserID = ""
		rsp := test.DB.Save(test.Data.firstOrder)
		require.NoError(t, rsp.Error, "Failed to update email")

		token := testToken("villian", "villian@wayneindustries.com")
		recorder := test.TestEndpoint(http.MethodPost, "/claim", nil, token)
		require.Equal(t, http.StatusNoContent, recorder.Code)

		recorder = test.TestEndpoint(http.MethodPost, "/claim", nil, token)
		require.Equal(t, http.StatusNoContent, recorder.Code)
	})
}

// -------------------------------------------------------------------------------------------------------------------
// HELPERS
// -------------------------------------------------------------------------------------------------------------------

func runOrderUpdate(test *RouteTest, order *models.Order, params *orderRequestParams, token *jwt.Token) *httptest.ResponseRecorder {
	updateBody, err := json.Marshal(params)
	require.NoError(test.T, err, "Failed to marshal data for update")
	recorder := test.TestEndpoint(http.MethodPut, fmt.Sprintf("/orders/%s", order.ID), bytes.NewReader(updateBody), token)
	return recorder
}

// -------------------------------------------------------------------------------------------------------------------
// VALIDATORS
// -------------------------------------------------------------------------------------------------------------------

func validateOrder(t *testing.T, expected, actual *models.Order) {
	assert := assert.New(t)

	// all the stock fields
	assert.Equal(expected.ID, actual.ID)
	assert.Equal(expected.UserID, actual.UserID)
	assert.Equal(expected.Email, actual.Email)
	assert.Equal(expected.Currency, actual.Currency)
	assert.Equal(expected.Taxes, actual.Taxes)
	assert.Equal(expected.Shipping, actual.Shipping)
	assert.Equal(expected.SubTotal, actual.SubTotal)
	assert.Equal(expected.Total, actual.Total)
	assert.Equal(expected.PaymentState, actual.PaymentState)
	assert.Equal(expected.FulfillmentState, actual.FulfillmentState)
	assert.Equal(expected.State, actual.State)
	assert.Equal(expected.ShippingAddressID, actual.ShippingAddressID)
	assert.Equal(expected.BillingAddressID, actual.BillingAddressID)
	assert.WithinDuration(expected.CreatedAt, actual.CreatedAt, time.Duration(1)*time.Second)
	assert.WithinDuration(expected.UpdatedAt, actual.UpdatedAt, time.Duration(1)*time.Second)
	assert.Equal(expected.VATNumber, actual.VATNumber)

	// we don't return the actual user
	assert.Nil(actual.User)

	for _, exp := range expected.LineItems {
		found := false
		for _, act := range actual.LineItems {
			if act.ID == exp.ID {
				found = true
				// We must JSON compare here because we sometimes validate
				// using values returned from an HTTP endpoint, which omits
				// certain fields
				expJSON, err := json.Marshal(exp)
				require.NoError(t, err)
				actJSON, err := json.Marshal(act)
				require.NoError(t, err)
				assert.JSONEq(string(expJSON), string(actJSON))
			}
		}
		assert.True(found, "Failed to find line item: %d %+v", exp.ID, actual.LineItems)
	}
}

func validateAllOrders(t *testing.T, actual []models.Order, expected *TestData) {
	for _, o := range actual {
		switch o.ID {
		case expected.firstOrder.ID:
			validateOrder(t, expected.firstOrder, &o)
			validateAddress(t, expected.firstOrder.BillingAddress, o.BillingAddress)
			validateAddress(t, expected.firstOrder.ShippingAddress, o.ShippingAddress)
		case expected.secondOrder.ID:
			validateOrder(t, expected.secondOrder, &o)
			validateAddress(t, expected.secondOrder.BillingAddress, o.BillingAddress)
			validateAddress(t, expected.secondOrder.ShippingAddress, o.ShippingAddress)
		default:
			assert.Fail(t, fmt.Sprintf("unexpected order: %+v\n", o))
		}
	}
}

func validateNewUserEmail(t *testing.T, order *models.Order, claims *claims.JWTClaims, expectedUserEmail, expectedOrderEmail string) {
	db, _, _, _ := db(t)
	result := db.First(new(models.User), "id = ?", claims.Subject)
	require.True(t, result.RecordNotFound(), "Unclean test env -- user exists with ID "+claims.Subject)

	err := setOrderEmail(db, order, claims, testLogger)
	require.NoError(t, err)

	user := new(models.User)
	result = db.First(user, "id = ?", claims.Subject)
	require.False(t, result.RecordNotFound())
	assert := assert.New(t)
	assert.Equal(claims.Subject, user.ID)
	assert.Equal(claims.Subject, order.UserID)
	assert.Equal(expectedOrderEmail, order.Email)
	assert.Equal(expectedUserEmail, user.Email)

	db.Unscoped().Delete(user)
	//t.Logf("Deleted user %s", claims.Subject)
}

func validateExistingUserEmail(t *testing.T, db *gorm.DB, order *models.Order, claims *claims.JWTClaims, expectedOrderEmail string) {
	require.NoError(t, setOrderEmail(db, order, claims, testLogger))
	assert.Equal(t, claims.Subject, order.UserID)
	assert.Equal(t, expectedOrderEmail, order.Email)
}

func startTestSite() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/simple-product":
			fmt.Fprintln(w, `<!doctype html>
				<html>
				<head><title>Test Product</title></head>
				<body>
					<script class="gocommerce-product">
					{"sku": "product-1", "title": "Product 1", "type": "Book", "prices": [
						{"amount": "9.99", "currency": "USD"}
					]}
					</script>
				</body>
				</html>`)
		case "/bundle-product":
			fmt.Fprintln(w, `<!doctype html>
				<html>
				<head><title>Test Product</title></head>
				<body>
					<script class="gocommerce-product">
					{"sku": "product-1", "title": "Product 1", "type": "Book", "prices": [
						{"amount": "9.99", "currency": "USD", "items": [
							{"amount": "7.00", "type": "Book"},
							{"amount": "2.99", "type": "E-Book"}
						]}
					]}
					</script>
				</body>
				</html>`)
		case "/gocommerce/settings.json":
			fmt.Fprintln(w, `{
				"taxes": [
					{"percentage": 19, "product_types": ["E-Book"], "countries": ["Germany"]},
					{"percentage": 7, "product_types": ["Book"], "countries": ["Germany"]}
				]
			}`)
		}
	}))
}
