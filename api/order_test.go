package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/guregu/kami"
	"github.com/stretchr/testify/assert"

	"github.com/netlify/netlify-commerce/models"
)

// ------------------------------------------------------------------------------------------------
// LIST
// ------------------------------------------------------------------------------------------------
func TestOrderQueryForAllOrdersAsTheUser(t *testing.T) {
	db := db(t)

	config := testConfig()
	ctx := testContext(testToken(testUser.ID, testUser.Email, nil), config)
	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "https://not-real", nil)

	api := NewAPI(config, db, nil)
	api.OrderList(ctx, recorder, req)

	orders := []models.Order{}
	extractPayload(t, 200, recorder, &orders)
	assert.Equal(t, 2, len(orders))

	for _, o := range orders {
		switch o.ID {
		case firstOrder.ID:
			validateOrder(t, firstOrder, &o)
			validateAddress(t, firstOrder.BillingAddress, o.BillingAddress)
			validateAddress(t, firstOrder.ShippingAddress, o.ShippingAddress)
		case secondOrder.ID:
			validateOrder(t, secondOrder, &o)
			validateAddress(t, secondOrder.BillingAddress, o.BillingAddress)
			validateAddress(t, secondOrder.ShippingAddress, o.ShippingAddress)
		default:
			assert.Fail(t, fmt.Sprintf("unexpected order: %+v\n", o))
		}
	}
}

func TestOrderQueryForAllOrdersAsAdmin(t *testing.T) {
	db := db(t)

	config := testConfig()
	config.JWT.AdminGroupName = "admin"
	ctx := testContext(testToken("admin-yo", "admin@wayneindustries.com", &[]string{"admin"}), config)
	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", urlWithUserID, nil)

	api := NewAPI(config, db, nil)
	api.OrderList(ctx, recorder, req)
	orders := []models.Order{}
	extractPayload(t, 200, recorder, &orders)

	assert.Equal(t, 2, len(orders))
	for _, o := range orders {
		switch o.ID {
		case firstOrder.ID:
			validateOrder(t, firstOrder, &o)
			validateAddress(t, firstOrder.BillingAddress, o.BillingAddress)
			validateAddress(t, firstOrder.ShippingAddress, o.ShippingAddress)
		case secondOrder.ID:
			validateOrder(t, secondOrder, &o)
			validateAddress(t, secondOrder.BillingAddress, o.BillingAddress)
			validateAddress(t, secondOrder.ShippingAddress, o.ShippingAddress)
		default:
			assert.Fail(t, fmt.Sprintf("unexpected order: %+v\n", o))
		}
	}
}

func TestOrderQueryForAllOrdersAsStranger(t *testing.T) {
	db := db(t)

	config := testConfig()
	ctx := testContext(testToken("stranger", "stranger-danger@wayneindustries.com", nil), config)
	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "https://not-real", nil)

	api := NewAPI(config, db, nil)
	api.OrderList(ctx, recorder, req)
	assert.Equal(t, 200, recorder.Code)
	assert.Equal(t, "[]\n", recorder.Body.String())
}

func TestOrderQueryForAllOrdersNotWithAdminRights(t *testing.T) {
	db := db(t)
	config := testConfig()
	ctx := testContext(testToken("stranger", "stranger-danger@wayneindustries.com", nil), config)

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", urlWithUserID, nil)

	api := NewAPI(config, db, nil)
	api.OrderList(ctx, recorder, req)
	assert.Equal(t, 400, recorder.Code)
	validateError(t, 400, recorder)
}

func TestOrderQueryForAllOrdersWithNoToken(t *testing.T) {
	config := testConfig()
	ctx := testContext(nil, config)

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", urlWithUserID, nil)

	api := NewAPI(config, nil, nil)
	api.OrderList(ctx, recorder, req)
	assert.Equal(t, 401, recorder.Code)
	validateError(t, 401, recorder)
}

// -------------------------------------------------------------------------------------------------------------------
// VIEW
// -------------------------------------------------------------------------------------------------------------------

func TestOrderQueryForAnOrderAsTheUser(t *testing.T) {
	db := db(t)
	config := testConfig()
	ctx := testContext(testToken(testUser.ID, "marp@wayneindustries.com", nil), config)

	// have to add it to the context ~ it isn't from the params
	ctx = kami.SetParam(ctx, "id", firstOrder.ID)

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "https://not-real/"+firstOrder.ID, nil)

	NewAPI(config, db, nil).OrderView(ctx, recorder, req)
	order := new(models.Order)
	extractPayload(t, 200, recorder, order)
	validateOrder(t, firstOrder, order)
	validateAddress(t, firstOrder.BillingAddress, order.BillingAddress)
	validateAddress(t, firstOrder.ShippingAddress, order.ShippingAddress)
}

func TestOrderQueryForAnOrderAsAnAdmin(t *testing.T) {
	db := db(t)
	config := testConfig()
	config.JWT.AdminGroupName = "admin"
	ctx := testContext(testToken("admin-yo", "admin@wayneindustries.com", &[]string{"admin"}), config)

	// have to add it to the context ~ it isn't from the params
	ctx = kami.SetParam(ctx, "id", firstOrder.ID)

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", urlForFirstOrder, nil)

	NewAPI(config, db, nil).OrderView(ctx, recorder, req)
	order := new(models.Order)
	extractPayload(t, 200, recorder, order)
	validateOrder(t, firstOrder, order)
	validateAddress(t, firstOrder.BillingAddress, order.BillingAddress)
	validateAddress(t, firstOrder.ShippingAddress, order.ShippingAddress)
}

func TestOrderQueryForAnOrderAsAStranger(t *testing.T) {
	db := db(t)
	config := testConfig()
	ctx := testContext(testToken("stranger", "stranger-danger@wayneindustries.com", nil), config)

	// have to add it to the context ~ it isn't from the params
	ctx = kami.SetParam(ctx, "id", firstOrder.ID)

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", urlForFirstOrder, nil)

	NewAPI(config, db, nil).OrderView(ctx, recorder, req)
	assert.Equal(t, 401, recorder.Code)
	validateError(t, 401, recorder)
}

func TestOrderQueryForAMissingOrder(t *testing.T) {
	db := db(t)
	config := testConfig()
	ctx := testContext(testToken("stranger", "stranger-danger@wayneindustries.com", nil), config)

	// have to add it to the context ~ it isn't from the params
	ctx = kami.SetParam(ctx, "id", "does-not-exist")

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "https://not-real/does-not-exist", nil)

	NewAPI(config, db, nil).OrderView(ctx, recorder, req)
	validateError(t, 404, recorder)
}

func TestOrderQueryForAnOrderWithNoToken(t *testing.T) {
	config := testConfig()
	ctx := testContext(nil, config)

	// have to add it to the context ~ it isn't from the params
	ctx = kami.SetParam(ctx, "id", "does-not-exist")

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "https://not-real/does-not-exist", nil)

	// use nil for DB b/c it should *NEVER* be called
	NewAPI(config, nil, nil).OrderView(ctx, recorder, req)
	validateError(t, 401, recorder)
}

// -------------------------------------------------------------------------------------------------------------------
// HELPERS
// -------------------------------------------------------------------------------------------------------------------

//func runUpdate(t *testing.T, db *gorm.DB, order *models.Order, params *OrderParams) *httptest.ResponseRecorder {
//	config := testConfig()
//	config.JWT.AdminGroupName = "admin"
//	ctx := testContext(token("admin-yo", "admin@wayneindustries.com", &[]string{"admin"}), config)
//	ctx = kami.SetParam(ctx, "id", order.ID)
//
//	updateBody, err := json.Marshal(params)
//	if !assert.NoError(t, err) {
//		assert.FailNow(t, "Failed to setup test "+err.Error())
//	}
//
//	recorder := httptest.NewRecorder()
//	req, _ := http.NewRequest("POST", fmt.Sprintf("https://not-real/%s", order.ID), bytes.NewReader(updateBody))
//
//	NewAPI(config, db, nil).OrderUpdate(ctx, recorder, req)
//	return recorder
//}

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
	assert.Equal(expected.CreatedAt.Unix(), actual.CreatedAt.Unix())
	assert.Equal(expected.UpdatedAt.Unix(), actual.UpdatedAt.Unix())
	assert.Equal(expected.VATNumber, actual.VATNumber)

	// we don't return the actual user
	assert.Nil(actual.User)

	for _, exp := range expected.LineItems {
		found := false
		for _, act := range expected.LineItems {
			if act.ID == exp.ID {
				found = true
				assert.Equal(exp, act)
			}
		}
		assert.True(found, fmt.Sprintf("Failed to find line item: %d", exp.ID))
	}
}

//func validateLineItem(t *testing.T, expected *models.LineItem, actual *models.LineItem) {
//	assert := assert.New(t)
//
//	assert.Equal(expected.ID, actual.ID)
//	assert.Equal(expected.Title, actual.Title)
//	assert.Equal(expected.SKU, actual.SKU)
//	assert.Equal(expected.Type, actual.Type)
//	assert.Equal(expected.Description, actual.Description)
//	assert.Equal(expected.VAT, actual.VAT)
//	assert.Equal(expected.Path, actual.Path)
//	assert.Equal(expected.Price, actual.Price)
//	assert.Equal(expected.Quantity, actual.Quantity)
//}

//func validateNewUserEmail(t *testing.T, order *models.Order, claims *JWTClaims, expectedUserEmail, expectedOrderEmail string) {
//	db := db(t)
//	assert := assert.New(t)
//	result := db.First(new(models.User), "id = ?", claims.ID)
//	if !result.RecordNotFound() {
//		assert.FailNow("Unclean test env -- user exists with ID " + claims.ID)
//	}
//
//	err := setOrderEmail(db, order, claims, testLogger)
//	if assert.NoError(err) {
//		user := new(models.User)
//		result = db.First(user, "id = ?", claims.ID)
//		assert.False(result.RecordNotFound())
//		assert.Equal(claims.ID, user.ID)
//		assert.Equal(claims.ID, order.UserID)
//		assert.Equal(expectedOrderEmail, order.Email)
//		assert.Equal(expectedUserEmail, user.Email)
//
//		db.Unscoped().Delete(user)
//		t.Logf("Deleted user %s", claims.ID)
//	}
//}

//func validateExistingUserEmail(t *testing.T, db *gorm.DB, order *models.Order, claims *JWTClaims, expectedOrderEmail string) {
//	assert := assert.New(t)
//	err := setOrderEmail(db, order, claims, testLogger)
//	if assert.NoError(err) {
//		assert.Equal(claims.ID, order.UserID)
//		assert.Equal(expectedOrderEmail, order.Email)
//	}
//}
