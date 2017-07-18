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

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"

	"github.com/netlify/gocommerce/claims"
	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/models"
	"github.com/stretchr/testify/require"
)

// ------------------------------------------------------------------------------------------------
// CREATE
// ------------------------------------------------------------------------------------------------
func TestOrderCreationWithSimpleOrder(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(nil, config, false)

	startTestSite(config)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "https://example.com/orders", strings.NewReader(`{
		"email": "info@example.com",
		"shipping_address": {
			"first_name": "Test", "last_name": "User",
			"address1": "610 22nd Street",
			"city": "San Francisco", "state": "CA", "country": "USA", "zip": "94107"
		},
		"line_items": [{"path": "/simple-product", "quantity": 1, "meta": {"attendees": [{"name": "Matt", "email": "matt@example.com"}]}}]
	}`)).WithContext(ctx)
	api := NewAPI(globalConfig, config, db)
	api.handler.ServeHTTP(recorder, req)

	order := &models.Order{}
	extractPayload(t, http.StatusCreated, recorder, order)

	var total uint64 = 999
	assert.Equal(t, "info@example.com", order.Email, "Total should be info@example.com, was %v", order.Email)
	assert.Equal(t, total, order.Total, fmt.Sprintf("Total should be 999, was %v", order.Total))
	if len(order.LineItems) != 1 {
		t.Errorf("Expected one item, got %v", len(order.LineItems))
	}
	meta := order.LineItems[0].MetaData
	if meta == nil {
		t.Error("Expected meta data for line item")
	}

	_, ok := meta["attendees"]
	if !ok {
		t.Error("Line item should have attendees")
	}
}

func TestOrderCreationWithTaxes(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(nil, config, false)

	startTestSite(config)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "https://example.com/orders", strings.NewReader(`{
		"email": "info@example.com",
		"shipping_address": {
			"first_name": "Test", "last_name": "User",
			"address1": "Branengebranen",
			"city": "Berlin", "country": "Germany", "zip": "94107"
		},
		"line_items": [{"path": "/simple-product", "quantity": 1}]
	}`)).WithContext(ctx)
	api := NewAPI(globalConfig, config, db)
	api.handler.ServeHTTP(recorder, req)

	order := &models.Order{}
	extractPayload(t, http.StatusCreated, recorder, order)

	var total uint64 = 1069
	var taxes uint64 = 70
	assert.Equal(t, "info@example.com", order.Email, "Total should be info@example.com, was %v", order.Email)
	assert.Equal(t, "Germany", order.ShippingAddress.Country)
	assert.Equal(t, "Germany", order.BillingAddress.Country)
	assert.Equal(t, total, order.Total, fmt.Sprintf("Total should be 1069, was %v", order.Total))
	assert.Equal(t, taxes, order.Taxes, fmt.Sprintf("Total should be 70, was %v", order.Total))
}

func TestOrderCreationForBundleWithTaxes(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(nil, config, false)
	startTestSite(config)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "https://example.com/orders", strings.NewReader(`{
		"email": "info@example.com",
		"shipping_address": {
			"first_name": "Test", "last_name": "User",
			"address1": "Branengebranen",
			"city": "Berlin", "country": "Germany", "zip": "94107"
		},
		"line_items": [{"path": "/bundle-product", "quantity": 1}]
	}`)).WithContext(ctx)
	api := NewAPI(globalConfig, config, db)
	api.handler.ServeHTTP(recorder, req)

	order := &models.Order{}
	extractPayload(t, http.StatusCreated, recorder, order)

	var total uint64 = 1105
	var taxes uint64 = 106
	assert.Equal(t, "info@example.com", order.Email, "Total should be info@example.com, was %v", order.Email)
	assert.Equal(t, "Germany", order.ShippingAddress.Country)
	assert.Equal(t, "Germany", order.BillingAddress.Country)
	assert.Equal(t, total, order.Total, fmt.Sprintf("Total should be 1105, was %v", order.Total))
	assert.Equal(t, taxes, order.Taxes, fmt.Sprintf("Total should be 106, was %v", order.Total))
}

// ------------------------------------------------------------------------------------------------
// LIST
// ------------------------------------------------------------------------------------------------
func TestOrderQueryForAllOrdersAsTheUser(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(nil, config, false)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://example.com/orders", nil).WithContext(ctx)

	token := testToken(testUser.ID, testUser.Email)
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	api := NewAPI(globalConfig, config, db)
	api.handler.ServeHTTP(recorder, req)

	orders := []models.Order{}
	extractPayload(t, http.StatusOK, recorder, &orders)
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

func TestOrderQueryEmailFilterAsTheUser(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(nil, config, false)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://example.com/orders?email=bruce", nil).WithContext(ctx)

	token := testToken(testUser.ID, testUser.Email)
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	api := NewAPI(globalConfig, config, db)
	api.handler.ServeHTTP(recorder, req)

	orders := []models.Order{}
	extractPayload(t, http.StatusOK, recorder, &orders)
	assert.Equal(t, 2, len(orders))
}

func TestEmptyOrderQueryEmailFilterAsTheUser(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(nil, config, false)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://example.com/orders?email=gmail.com", nil).WithContext(ctx)

	token := testToken(testUser.ID, testUser.Email)
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	api := NewAPI(globalConfig, config, db)
	api.handler.ServeHTTP(recorder, req)

	orders := []models.Order{}
	extractPayload(t, http.StatusOK, recorder, &orders)
	assert.Equal(t, 0, len(orders))
}

func TestOrderQueryItemFilterAsTheUser(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(nil, config, false)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://example.com/orders?items=batwing", nil).WithContext(ctx)

	token := testToken(testUser.ID, testUser.Email)
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	api := NewAPI(globalConfig, config, db)
	api.handler.ServeHTTP(recorder, req)

	orders := []models.Order{}
	extractPayload(t, http.StatusOK, recorder, &orders)
	assert.Equal(t, 1, len(orders))
}

func TestOrderQueryForAllOrdersAsAdmin(t *testing.T) {
	db, globalConfig, config := db(t)

	ctx := testContext(nil, config, true)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://example.com/users/all/orders", nil).WithContext(ctx)

	token := testAdminToken("admin-yo", "admin@wayneindustries.com")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	api := NewAPI(globalConfig, config, db)
	api.handler.ServeHTTP(recorder, req)

	orders := []models.Order{}
	extractPayload(t, http.StatusOK, recorder, &orders)

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

func TestOrderQueryForOwnOrdersAsStranger(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(nil, config, false)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://example.com/orders", nil).WithContext(ctx)

	token := testToken("stranger", "stranger-danger@wayneindustries.com")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	api := NewAPI(globalConfig, config, db)
	api.handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "[]", recorder.Body.String())
}

func TestOrderQueryForAllOrdersNotWithAdminRights(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(nil, config, false)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://example.com/users/all/orders", nil).WithContext(ctx)

	token := testToken("stranger", "stranger-danger@wayneindustries.com")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	api := NewAPI(globalConfig, config, db)
	api.handler.ServeHTTP(recorder, req)

	validateError(t, http.StatusUnauthorized, recorder)
}

func TestOrderQueryForAllOrdersWithNoToken(t *testing.T) {
	globalConfig, config := testConfig()
	ctx := testContext(nil, config, false)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://example.com/users/all/orders", nil).WithContext(ctx)

	api := NewAPI(globalConfig, config, nil)
	api.handler.ServeHTTP(recorder, req)

	validateError(t, http.StatusUnauthorized, recorder)
}

// -------------------------------------------------------------------------------------------------------------------
// VIEW
// -------------------------------------------------------------------------------------------------------------------

func TestOrderQueryForAnOrderAsTheUser(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(nil, config, false)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://example.com/orders/"+firstOrder.ID, nil).WithContext(ctx)

	token := testToken(testUser.ID, "marp@wayneindustries.com")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	api := NewAPI(globalConfig, config, db)
	api.handler.ServeHTTP(recorder, req)

	order := new(models.Order)
	extractPayload(t, http.StatusOK, recorder, order)
	validateOrder(t, firstOrder, order)
	validateAddress(t, firstOrder.BillingAddress, order.BillingAddress)
	validateAddress(t, firstOrder.ShippingAddress, order.ShippingAddress)
}

func TestOrderQueryForAnOrderAsAnAdmin(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(nil, config, true)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", urlForFirstOrder, nil).WithContext(ctx)

	token := testAdminToken("admin-yo", "admin@wayneindustries.com")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)
	order := new(models.Order)
	extractPayload(t, http.StatusOK, recorder, order)
	validateOrder(t, firstOrder, order)
	validateAddress(t, firstOrder.BillingAddress, order.BillingAddress)
	validateAddress(t, firstOrder.ShippingAddress, order.ShippingAddress)
}

func TestOrderQueryForAnOrderAsAStranger(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(nil, config, false)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", urlForFirstOrder, nil).WithContext(ctx)

	token := testToken("stranger", "stranger-danger@wayneindustries.com")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)
	validateError(t, http.StatusUnauthorized, recorder)
}

func TestOrderQueryForAMissingOrder(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(nil, config, false)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://example.com/orders/does-not-exist", nil).WithContext(ctx)

	token := testToken("stranger", "stranger-danger@wayneindustries.com")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)
	validateError(t, http.StatusNotFound, recorder)
}

func TestOrderQueryForAnOrderWithNoToken(t *testing.T) {
	globalConfig, config := testConfig()
	ctx := testContext(nil, config, false)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://example.com/orders/does-not-exist", nil).WithContext(ctx)

	NewAPI(globalConfig, config, nil).handler.ServeHTTP(recorder, req)
	validateError(t, http.StatusUnauthorized, recorder)
}

// --------------------------------------------------------------------------------------------------------------------
// Create ~ email logic
// --------------------------------------------------------------------------------------------------------------------
func TestOrderSetUserIDLogic_AnonymousUser(t *testing.T) {
	assert := assert.New(t)
	simpleOrder := models.NewOrder("session", "params@email.com", "usd")
	err := setOrderEmail(nil, simpleOrder, nil, testLogger)
	assert.Nil(err)
	assert.Equal("params@email.com", simpleOrder.Email)
}

func TestOrderSetUserIDLogic_AnonymousUserWithNoEmail(t *testing.T) {
	assert := assert.New(t)
	simpleOrder := models.NewOrder("session", "", "usd")
	err := setOrderEmail(nil, simpleOrder, nil, testLogger)
	if !assert.Error(err) {
		assert.Equal(http.StatusBadRequest, err.Code)
	}
}

func TestOrderSetUserIDLogic_NewUserNoEmailOnRequest(t *testing.T) {
	validateNewUserEmail(
		t,
		models.NewOrder("session", "", "usd"),
		testToken("alfred", "alfred@wayne.com").Claims.(*claims.JWTClaims),
		"alfred@wayne.com",
		"alfred@wayne.com",
	)
}

func TestOrderSetUserIDLogic_NewUserNoEmailOnClaim(t *testing.T) {
	validateNewUserEmail(
		t,
		models.NewOrder("session", "joker@wayne.com", "usd"),
		testToken("alfred", "").Claims.(*claims.JWTClaims),
		"",
		"joker@wayne.com",
	)
}

func TestOrderSetUserIDLogic_NewUserAllTheEmails(t *testing.T) {
	validateNewUserEmail(
		t,
		models.NewOrder("session", "joker@wayne.com", "usd"),
		testToken("alfred", "alfred@wayne.com").Claims.(*claims.JWTClaims),
		"alfred@wayne.com",
		"joker@wayne.com",
	)
}

func TestOrderSetUserIDLogic_NewUserNoEmails(t *testing.T) {
	db, _, _ := db(t)
	assert := assert.New(t)
	simpleOrder := models.NewOrder("session", "", "usd")
	claims := testToken("alfred", "").Claims.(*claims.JWTClaims)
	err := setOrderEmail(db, simpleOrder, claims, testLogger)
	if assert.Error(err) {
		assert.Equal(http.StatusBadRequest, err.Code)
	}
}

func TestOrderSetUserIDLogic_KnownUserClaimsOnRequest(t *testing.T) {
	db, _, _ := db(t)
	validateExistingUserEmail(
		t,
		db,
		models.NewOrder("session", "joker@wayne.com", "usd"),
		testToken(testUser.ID, "").Claims.(*claims.JWTClaims),
		"joker@wayne.com",
	)
}

func TestOrderSetUserIDLogic_KnownUserClaimsOnClaim(t *testing.T) {
	db, _, _ := db(t)
	validateExistingUserEmail(
		t,
		db,
		models.NewOrder("session", "", "usd"),
		testToken(testUser.ID, testUser.Email).Claims.(*claims.JWTClaims),
		testUser.Email,
	)
}

func TestOrderSetUserIDLogic_KnownUserAllTheEmail(t *testing.T) {
	db, _, _ := db(t)
	validateExistingUserEmail(
		t,
		db,
		models.NewOrder("session", "joker@wayne.com", "usd"),
		testToken(testUser.ID, testUser.Email).Claims.(*claims.JWTClaims),
		"joker@wayne.com",
	)
}

func TestOrderSetUserIDLogic_KnownUserNoEmail(t *testing.T) {
	db, _, _ := db(t)
	validateExistingUserEmail(
		t,
		db,
		models.NewOrder("session", "", "usd"),
		testToken(testUser.ID, "").Claims.(*claims.JWTClaims),
		testUser.Email,
	)
}

// --------------------------------------------------------------------------------------------------------------------
// UPDATE
// --------------------------------------------------------------------------------------------------------------------
func TestOrderUpdateFields(t *testing.T) {
	db, _, _ := db(t)
	defer db.Save(firstOrder)
	assert := assert.New(t)

	recorder := runUpdate(t, db, firstOrder, &orderRequestParams{
		Email:    "mrfreeze@dc.com",
		Currency: "monopoly-dollars",
	})
	rspOrder := new(models.Order)
	extractPayload(t, http.StatusOK, recorder, rspOrder)

	saved := new(models.Order)
	rsp := db.First(saved, "id = ?", firstOrder.ID)
	assert.False(rsp.RecordNotFound())

	assert.Equal("mrfreeze@dc.com", rspOrder.Email)
	assert.Equal("monopoly-dollars", saved.Currency)

	// did it get persisted to the db
	assert.Equal("mrfreeze@dc.com", saved.Email)
	assert.Equal("monopoly-dollars", saved.Currency)
	validateOrder(t, saved, rspOrder)

	// should be the only field that has changed ~ check it
	saved.Email = firstOrder.Email
	saved.Currency = firstOrder.Currency
	validateOrder(t, firstOrder, saved)
}

func TestOrderUpdateAddress_ExistingAddress(t *testing.T) {
	db, _, _ := db(t)
	defer db.Save(firstOrder)
	assert := assert.New(t)

	newAddr := getTestAddress()
	newAddr.ID = "new-addr"
	newAddr.UserID = firstOrder.UserID
	db.Create(newAddr)
	defer db.Unscoped().Delete(newAddr)

	recorder := runUpdate(t, db, firstOrder, &orderRequestParams{
		BillingAddressID: newAddr.ID,
	})
	rspOrder := new(models.Order)
	extractPayload(t, http.StatusOK, recorder, rspOrder)

	saved := new(models.Order)
	rsp := db.First(saved, "id = ?", firstOrder.ID)
	assert.False(rsp.RecordNotFound())

	// now we load the addresses
	assert.Equal(saved.BillingAddressID, rspOrder.BillingAddressID)

	savedAddr := &models.Address{ID: saved.BillingAddressID}
	rsp = db.First(savedAddr)
	assert.False(rsp.RecordNotFound())
	defer db.Unscoped().Delete(savedAddr)

	validateAddress(t, *newAddr, *savedAddr)
}

func TestOrderUpdateAddress_NewAddress(t *testing.T) {
	db, _, _ := db(t)
	defer db.Save(firstOrder)
	assert := assert.New(t)

	paramsAddress := getTestAddress()
	recorder := runUpdate(t, db, firstOrder, &orderRequestParams{
		// should create a new address associated with the order's user
		ShippingAddress: paramsAddress,
	})
	rspOrder := new(models.Order)
	extractPayload(t, http.StatusOK, recorder, rspOrder)

	saved := new(models.Order)
	rsp := db.First(saved, "id = ?", firstOrder.ID)
	assert.False(rsp.RecordNotFound())

	// now we load the addresses
	assert.Equal(saved.ShippingAddressID, rspOrder.ShippingAddressID)

	savedAddr := &models.Address{ID: saved.ShippingAddressID}
	rsp = db.First(savedAddr)
	assert.False(rsp.RecordNotFound())
	defer db.Unscoped().Delete(savedAddr)

	validateAddress(t, *paramsAddress, *savedAddr)
}

func TestOrderUpdateAsNonAdmin(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(nil, config, false)

	params := &orderRequestParams{
		Email:    "mrfreeze@dc.com",
		Currency: "monopoly-dollars",
	}

	updateBody, err := json.Marshal(params)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", urlForFirstOrder, bytes.NewReader(updateBody)).WithContext(ctx)

	token := testToken("villian", "villian@wayneindustries.com")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)
	validateError(t, http.StatusUnauthorized, recorder)
}

func TestOrderUpdateWithNoCreds(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(nil, config, false)

	params := &orderRequestParams{
		Email:    "mrfreeze@dc.com",
		Currency: "monopoly-dollars",
	}

	updateBody, err := json.Marshal(params)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", urlForFirstOrder, bytes.NewReader(updateBody)).WithContext(ctx)

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)
	validateError(t, http.StatusUnauthorized, recorder)
}

func TestOrderUpdateWithNewData(t *testing.T) {
	db, _, _ := db(t)
	assert := assert.New(t)
	op := &orderRequestParams{
		MetaData: map[string]interface{}{
			"thing":       float64(1),
			"red":         "fish",
			"other thing": 3.4,
			"exists":      true,
		},
	}
	recorder := runUpdate(t, db, firstOrder, op)
	order := &models.Order{}
	extractPayload(t, http.StatusOK, recorder, order)

	assert.Equal(op.MetaData, order.MetaData, "Order metadata should have been updated")
}

// -------------------------------------------------------------------------------------------------------------------
// CLAIMS
// -------------------------------------------------------------------------------------------------------------------

func TestClaimOrders(t *testing.T) {
	db, globalConfig, config := db(t)

	firstOrder.Email = "villian@wayneindustries.com"
	firstOrder.UserID = ""
	if rsp := db.Save(firstOrder); rsp.Error != nil {
		assert.FailNow(t, "Failed to update email: %v", rsp.Error)
	}

	ctx := testContext(nil, config, false)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "https://example.com/claim", nil).WithContext(ctx)

	token := testToken("villian", "villian@wayneindustries.com")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusNoContent, recorder.Code)

	// validate the DB
	dbOrders := []models.Order{}
	if rsp := db.Where("email = ?", "villian@wayneindustries.com").Find(&dbOrders); rsp.Error != nil {
		assert.FailNow(t, "Failed to query DB: "+rsp.Error.Error())
	}

	assert.Len(t, dbOrders, 1)
	assert.Equal(t, "villian@wayneindustries.com", dbOrders[0].Email)
	assert.Equal(t, "villian", dbOrders[0].UserID)
}

func TestClaimOrdersNoEmail(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(nil, config, false)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "https://example.com/claim", nil).WithContext(ctx)

	token := testToken("villian", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)
	validateError(t, http.StatusBadRequest, recorder)
}

func TestClaimOrdersNoID(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(nil, config, false)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "https://example.com/claim", nil).WithContext(ctx)

	token := testToken("", "villian@wayneindustries.com")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)
	validateError(t, http.StatusBadRequest, recorder)
}

func TestClaimOrdersMultipleTimes(t *testing.T) {
	db, globalConfig, config := db(t)

	firstOrder.Email = "villian@wayneindustries.com"
	firstOrder.UserID = ""
	if rsp := db.Save(firstOrder); rsp.Error != nil {
		assert.FailNow(t, "Failed to update email: %v", rsp.Error)
	}

	ctx := testContext(nil, config, false)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "https://example.com/claim", nil).WithContext(ctx)

	token := testToken("villian", "villian@wayneindustries.com")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	api := NewAPI(globalConfig, config, db)
	api.handler.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusNoContent, recorder.Code)

	// run it again
	recorder = httptest.NewRecorder()
	api.handler.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

// -------------------------------------------------------------------------------------------------------------------
// HELPERS
// -------------------------------------------------------------------------------------------------------------------

func runUpdate(t *testing.T, db *gorm.DB, order *models.Order, params *orderRequestParams) *httptest.ResponseRecorder {
	globalConfig, config := testConfig()
	ctx := testContext(nil, config, true)

	updateBody, err := json.Marshal(params)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Failed to setup test "+err.Error())
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", fmt.Sprintf("https://example.com/orders/%s", order.ID), bytes.NewReader(updateBody)).WithContext(ctx)

	token := testAdminToken("admin-yo", "admin@wayneindustries.com")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)
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
//	assert.Equal(expected.Sku, actual.Sku)
//	assert.Equal(expected.Type, actual.Type)
//	assert.Equal(expected.Description, actual.Description)
//	assert.Equal(expected.VAT, actual.VAT)
//	assert.Equal(expected.Path, actual.Path)
//	assert.Equal(expected.Price, actual.Price)
//	assert.Equal(expected.Quantity, actual.Quantity)
//}

func validateNewUserEmail(t *testing.T, order *models.Order, claims *claims.JWTClaims, expectedUserEmail, expectedOrderEmail string) {
	db, _, _ := db(t)
	assert := assert.New(t)
	result := db.First(new(models.User), "id = ?", claims.ID)
	if !result.RecordNotFound() {
		assert.FailNow("Unclean test env -- user exists with ID " + claims.ID)
	}

	err := setOrderEmail(db, order, claims, testLogger)
	if assert.NoError(err) {
		user := new(models.User)
		result = db.First(user, "id = ?", claims.ID)
		assert.False(result.RecordNotFound())
		assert.Equal(claims.ID, user.ID)
		assert.Equal(claims.ID, order.UserID)
		assert.Equal(expectedOrderEmail, order.Email)
		assert.Equal(expectedUserEmail, user.Email)

		db.Unscoped().Delete(user)
		t.Logf("Deleted user %s", claims.ID)
	}
}

func validateExistingUserEmail(t *testing.T, db *gorm.DB, order *models.Order, claims *claims.JWTClaims, expectedOrderEmail string) {
	assert := assert.New(t)
	err := setOrderEmail(db, order, claims, testLogger)
	if assert.NoError(err) {
		assert.Equal(claims.ID, order.UserID)
		assert.Equal(expectedOrderEmail, order.Email)
	}
}

func startTestSite(globalConfig *conf.Configuration) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	globalConfig.SiteURL = ts.URL
}
