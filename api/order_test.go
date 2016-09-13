package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/dgrijalva/jwt-go"
	"github.com/guregu/kami"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/stretchr/testify/assert"

	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/models"
)

var testLogger = logrus.NewEntry(logrus.StandardLogger())
var config = &conf.Configuration{}
var db *gorm.DB

var urlWithUserID string
var urlForFirstOrder string

func TestMain(m *testing.M) {
	f, err := ioutil.TempFile("", "test-db")
	if err != nil {
		panic(err)
	}
	defer os.Remove(f.Name())

	config.DB.Driver = "sqlite3"
	config.DB.ConnURL = f.Name()

	// setup test db
	db, err = models.Connect(config)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	loadTestData(db)
	urlForFirstOrder = fmt.Sprintf("https://not-real/%s", firstOrder.ID)
	urlWithUserID = fmt.Sprintf("https://not-real?user_id=%s", testUser.ID)

	os.Exit(m.Run())
}

// -------------------------------------------------------------------------------------------------------------------
// LIST
// -------------------------------------------------------------------------------------------------------------------

func TestQueryForAllOrdersAsTheUser(t *testing.T) {
	a := assert.New(t)

	ctx := testContext(token(testUser.ID, testUser.Email, nil))
	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "https://not-real", nil)

	api := NewAPI(config, db, nil)
	api.OrderList(ctx, recorder, req)

	orders := extractOrders(t, 200, recorder)
	a.Equal(2, len(orders))

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
			a.Fail(fmt.Sprintf("unexpected order: %+v\n", o))
		}
	}
}

func TestQueryForAllOrdersAsAdmin(t *testing.T) {
	a := assert.New(t)

	config.JWT.AdminGroupName = "admin"
	ctx := testContext(token("admin-yo", "admin@wayneindustries.com", &[]string{"admin"}))
	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", urlWithUserID, nil)

	api := NewAPI(config, db, nil)
	api.OrderList(ctx, recorder, req)
	orders := extractOrders(t, 200, recorder)

	a.Equal(2, len(orders))
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
			a.Fail(fmt.Sprintf("unexpected order: %+v\n", o))
		}
	}
}

func TestQueryForAllOrdersAsStranger(t *testing.T) {
	a := assert.New(t)

	ctx := testContext(token("stranger", "stranger-danger@wayneindustries.com", nil))
	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "https://not-real", nil)

	api := NewAPI(config, db, nil)
	api.OrderList(ctx, recorder, req)
	a.Equal(200, recorder.Code)
	a.Equal("[]\n", recorder.Body.String())
}

func TestQueryForAllOrdersNotWithAdminRights(t *testing.T) {
	a := assert.New(t)
	ctx := testContext(token("stranger", "stranger-danger@wayneindustries.com", nil))

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", urlWithUserID, nil)

	api := NewAPI(config, db, nil)
	api.OrderList(ctx, recorder, req)
	a.Equal(400, recorder.Code)
	validateError(t, 400, recorder.Body)
}

func TestQueryForAllOrdersWithNoToken(t *testing.T) {
	a := assert.New(t)
	ctx := testContext(nil)

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", urlWithUserID, nil)

	api := NewAPI(config, nil, nil)
	api.OrderList(ctx, recorder, req)
	a.Equal(401, recorder.Code)
	validateError(t, 401, recorder.Body)
}

// -------------------------------------------------------------------------------------------------------------------
// VIEW
// -------------------------------------------------------------------------------------------------------------------

func TestQueryForAnOrderAsTheUser(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	ctx := testContext(token(testUser.ID, "marp@wayneindustries.com", nil))

	// have to add it to the context ~ it isn't from the params
	ctx = kami.SetParam(ctx, "id", firstOrder.ID)

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "https://not-real/"+firstOrder.ID, nil)

	api := NewAPI(config, db, nil)
	api.OrderView(ctx, recorder, req)
	order := extractOrder(t, 200, recorder)
	validateOrder(t, firstOrder, order)
	validateAddress(t, firstOrder.BillingAddress, order.BillingAddress)
	validateAddress(t, firstOrder.ShippingAddress, order.ShippingAddress)
}

func TestQueryForAnOrderAsAnAdmin(t *testing.T) {
	config.JWT.AdminGroupName = "admin"
	ctx := testContext(token("admin-yo", "admin@wayneindustries.com", &[]string{"admin"}))

	// have to add it to the context ~ it isn't from the params
	ctx = kami.SetParam(ctx, "id", firstOrder.ID)

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", urlForFirstOrder, nil)

	api := NewAPI(config, db, nil)
	api.OrderView(ctx, recorder, req)
	order := extractOrder(t, 200, recorder)
	validateOrder(t, firstOrder, order)
	validateAddress(t, firstOrder.BillingAddress, order.BillingAddress)
	validateAddress(t, firstOrder.ShippingAddress, order.ShippingAddress)
}

func TestQueryForAnOrderAsAStranger(t *testing.T) {
	a := assert.New(t)
	ctx := testContext(token("stranger", "stranger-danger@wayneindustries.com", nil))

	// have to add it to the context ~ it isn't from the params
	ctx = kami.SetParam(ctx, "id", firstOrder.ID)

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", urlForFirstOrder, nil)

	api := NewAPI(config, db, nil)
	api.OrderView(ctx, recorder, req)
	a.Equal(401, recorder.Code)
	validateError(t, 401, recorder.Body)
}

func TestQueryForAMissingOrder(t *testing.T) {
	a := assert.New(t)
	ctx := testContext(token("stranger", "stranger-danger@wayneindustries.com", nil))

	// have to add it to the context ~ it isn't from the params
	ctx = kami.SetParam(ctx, "id", "does-not-exist")

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "https://not-real/does-not-exist", nil)

	api := NewAPI(config, db, nil)
	api.OrderView(ctx, recorder, req)
	a.Equal(404, recorder.Code)
	validateError(t, 404, recorder.Body)
}

func TestQueryForAnOrderWithNoToken(t *testing.T) {
	a := assert.New(t)
	ctx := testContext(nil)

	// have to add it to the context ~ it isn't from the params
	ctx = kami.SetParam(ctx, "id", "does-not-exist")

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "https://not-real/does-not-exist", nil)

	// use nil for DB b/c it should *NEVER* be called
	api := NewAPI(config, nil, nil)
	api.OrderView(ctx, recorder, req)
	a.Equal(401, recorder.Code)
	validateError(t, 401, recorder.Body)
}

// -------------------------------------------------------------------------------------------------------------------
// CREATE
// -------------------------------------------------------------------------------------------------------------------
// TODO vvvv ~ need to make it verifiable
//func TestCreateAnOrderAsAnExistingUser(t *testing.T) {
//	a := assert.New(t)
//	orderRequest := &OrderParams{
//		SessionID: "test-session",
//		LineItems: []*OrderLineItem{&OrderLineItem{
//			SKU:      "nananana",
//			Path:     "/fashion/utility-belt",
//			Quantity: 1,
//		}},
//		BillingAddress: &testAddress,
//		ShippingAddress: &models.Address{
//			LastName: "robin",
//			Address1: "123456 circus lane",
//			Country:  "dcland",
//			City:     "gotham",
//			Zip:      "234789",
//		},
//	}
//
//	bs, err := json.Marshal(orderRequest)
//	if !assert.NoError(t, err) {
//		assert.FailNow(t, "setup failure")
//	}
//
//	ctx := testContext(token(testUser.ID, testUser.Email, nil))
//	recorder := httptest.NewRecorder()
//	req, err := http.NewRequest("PUT", "https://not-real/orders", bytes.NewReader(bs))
//
//	api := NewAPI(config, db, nil)
//	api.OrderCreate(ctx, recorder, req)
//	a.Equal(200, recorder.Code)
//
//	//ret := new(models.Order)
//	ret := make(map[string]interface{})
//	err = json.Unmarshal(recorder.Body.Bytes(), ret)
//	a.NoError(err)
//
//	fmt.Printf("%+v\n", ret)
//}

// --------------------------------------------------------------------------------------------------------------------
// Create ~ email logic
// --------------------------------------------------------------------------------------------------------------------
func TestSetUserIDLogic_AnonymousUser(t *testing.T) {
	a := assert.New(t)
	simpleOrder := models.NewOrder("session", "params@email.com", "usd")
	err := setOrderEmail(nil, simpleOrder, nil, testLogger)
	a.Nil(err)
	a.Equal("params@email.com", simpleOrder.Email)
}

func TestSetUserIDLogic_AnonymousUserWithNoEmail(t *testing.T) {
	a := assert.New(t)
	simpleOrder := models.NewOrder("session", "", "usd")
	err := setOrderEmail(nil, simpleOrder, nil, testLogger)
	if !a.Error(err) {
		a.Equal(400, err.Code)
	}
}

func TestSetUserIDLogic_NewUserNoEmailOnRequest(t *testing.T) {
	validateNewUserEmail(
		t,
		models.NewOrder("session", "", "usd"),
		token("alfred", "alfred@wayne.com", nil).Claims.(*JWTClaims),
		"alfred@wayne.com",
		"alfred@wayne.com",
	)
}

func TestSetUserIDLogic_NewUserNoEmailOnClaim(t *testing.T) {
	validateNewUserEmail(
		t,
		models.NewOrder("session", "joker@wayne.com", "usd"),
		token("alfred", "", nil).Claims.(*JWTClaims),
		"",
		"joker@wayne.com",
	)
}

func TestSetUserIDLogic_NewUserAllTheEmails(t *testing.T) {
	validateNewUserEmail(
		t,
		models.NewOrder("session", "joker@wayne.com", "usd"),
		token("alfred", "alfred@wayne.com", nil).Claims.(*JWTClaims),
		"alfred@wayne.com",
		"joker@wayne.com",
	)
}

func TestSetUserIDLogic_NewUserNoEmails(t *testing.T) {
	a := assert.New(t)
	simpleOrder := models.NewOrder("session", "", "usd")
	claims := token("alfred", "", nil).Claims.(*JWTClaims)
	err := setOrderEmail(db, simpleOrder, claims, testLogger)
	if a.Error(err) {
		a.Equal(400, err.Code)
	}
}

func TestSetUserIDLogic_KnownUserClaimsOnRequest(t *testing.T) {
	validateExistingUserEmail(
		t,
		models.NewOrder("session", "joker@wayne.com", "usd"),
		token(testUser.ID, "", nil).Claims.(*JWTClaims),
		"joker@wayne.com",
	)
}

func TestSetUserIDLogic_KnownUserClaimsOnClaim(t *testing.T) {
	validateExistingUserEmail(
		t,
		models.NewOrder("session", "", "usd"),
		token(testUser.ID, testUser.Email, nil).Claims.(*JWTClaims),
		testUser.Email,
	)
}

func TestSetUserIDLogic_KnownUserAllTheEmail(t *testing.T) {
	validateExistingUserEmail(
		t,
		models.NewOrder("session", "joker@wayne.com", "usd"),
		token(testUser.ID, testUser.Email, nil).Claims.(*JWTClaims),
		"joker@wayne.com",
	)
}

func TestSetUserIDLogic_KnownUserNoEmail(t *testing.T) {
	validateExistingUserEmail(
		t,
		models.NewOrder("session", "", "usd"),
		token(testUser.ID, "", nil).Claims.(*JWTClaims),
		testUser.Email,
	)
}

// --------------------------------------------------------------------------------------------------------------------
// UPDATE
// --------------------------------------------------------------------------------------------------------------------
func TestUpdateFields(t *testing.T) {
	defer db.Save(firstOrder)
	a := assert.New(t)

	recorder := runUpdate(t, firstOrder, &OrderParams{
		Email:    "mrfreeze@dc.com",
		Currency: "monopoly-dollars",
	})
	rspOrder := extractOrder(t, 200, recorder)

	saved := new(models.Order)
	rsp := db.First(saved, "id = ?", firstOrder.ID)
	a.False(rsp.RecordNotFound())

	a.Equal("mrfreeze@dc.com", rspOrder.Email)
	a.Equal("monopoly-dollars", saved.Currency)

	// did it get persisted to the db
	a.Equal("mrfreeze@dc.com", saved.Email)
	a.Equal("monopoly-dollars", saved.Currency)
	validateOrder(t, saved, rspOrder)

	// should be the only field that has changed ~ check it
	saved.Email = firstOrder.Email
	saved.Currency = firstOrder.Currency
	validateOrder(t, firstOrder, saved)
}

func TestUpdateAddress_ExistingAddress(t *testing.T) {
	defer db.Save(firstOrder)
	a := assert.New(t)

	newAddr := getTestAddress()
	newAddr.ID = "new-addr"
	newAddr.UserID = firstOrder.UserID
	rsp := db.Create(newAddr)
	defer db.Unscoped().Delete(newAddr)

	recorder := runUpdate(t, firstOrder, &OrderParams{
		BillingAddressID: newAddr.ID,
	})
	rspOrder := extractOrder(t, 200, recorder)

	saved := new(models.Order)
	rsp = db.First(saved, "id = ?", firstOrder.ID)
	a.False(rsp.RecordNotFound())

	// now we load the addresses
	a.Equal(saved.BillingAddressID, rspOrder.BillingAddressID)

	savedAddr := &models.Address{ID: saved.BillingAddressID}
	rsp = db.First(savedAddr)
	a.False(rsp.RecordNotFound())
	defer db.Unscoped().Delete(savedAddr)

	validateAddress(t, *newAddr, *savedAddr)
}

func TestUpdateAddress_NewAddress(t *testing.T) {
	defer db.Save(firstOrder)
	a := assert.New(t)

	paramsAddress := getTestAddress()
	recorder := runUpdate(t, firstOrder, &OrderParams{
		// should create a new address associated with the order's user
		ShippingAddress: paramsAddress,
	})
	rspOrder := extractOrder(t, 200, recorder)

	saved := new(models.Order)
	rsp := db.First(saved, "id = ?", firstOrder.ID)
	a.False(rsp.RecordNotFound())

	// now we load the addresses
	a.Equal(saved.ShippingAddressID, rspOrder.ShippingAddressID)

	savedAddr := &models.Address{ID: saved.ShippingAddressID}
	rsp = db.First(savedAddr)
	a.False(rsp.RecordNotFound())
	defer db.Unscoped().Delete(savedAddr)

	validateAddress(t, *paramsAddress, *savedAddr)
}

func TestUpdatePaymentInfoAfterPaid(t *testing.T) {
	defer db.Save(firstOrder)
	db.Model(firstOrder).Update("payment_state", "paid")

	recorder := runUpdate(t, firstOrder, &OrderParams{
		Currency: "monopoly",
	})
	validateError(t, 400, recorder.Body)
}

func TestUpdateBillingAddressAfterPaid(t *testing.T) {
	defer db.Model(firstOrder).Update("payment_state", "pending")
	db.Model(firstOrder).Update("payment_state", "paid")

	recorder := runUpdate(t, firstOrder, &OrderParams{
		BillingAddressID: testAddress.ID,
	})
	validateError(t, 400, recorder.Body)
}

func TestUpdateShippingAfterShipped(t *testing.T) {
	defer db.Model(firstOrder).Update("fulfillment_state", "pending")
	db.Model(firstOrder).Update("fulfillment_state", "paid")

	recorder := runUpdate(t, firstOrder, &OrderParams{
		ShippingAddressID: testAddress.ID,
	})
	validateError(t, 400, recorder.Body)
}

func TestUpdateAsNonAdmin(t *testing.T) {
	ctx := testContext(token("villian", "villian@wayneindustries.com", nil))
	ctx = kami.SetParam(ctx, "id", firstOrder.ID)

	params := &OrderParams{
		Email:    "mrfreeze@dc.com",
		Currency: "monopoly-dollars",
	}

	updateBody, err := json.Marshal(params)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Failed to setup test "+err.Error())
	}

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", urlWithUserID, bytes.NewReader(updateBody))

	api := NewAPI(config, db, nil)
	api.OrderUpdate(ctx, recorder, req)
	validateError(t, 401, recorder.Body)
}

func TestUpdateWithNoCreds(t *testing.T) {
	ctx := testContext(nil)
	ctx = kami.SetParam(ctx, "id", firstOrder.ID)

	params := &OrderParams{
		Email:    "mrfreeze@dc.com",
		Currency: "monopoly-dollars",
	}

	updateBody, err := json.Marshal(params)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Failed to setup test "+err.Error())
	}

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", urlForFirstOrder, bytes.NewReader(updateBody))

	api := NewAPI(config, db, nil)
	api.OrderUpdate(ctx, recorder, req)
	validateError(t, 401, recorder.Body)
}

// -------------------------------------------------------------------------------------------------------------------
// HELPERS
// -------------------------------------------------------------------------------------------------------------------

func extractOrders(t *testing.T, code int, recorder *httptest.ResponseRecorder) []models.Order {
	assert.Equal(t, code, recorder.Code)
	orders := []models.Order{}
	err := json.NewDecoder(recorder.Body).Decode(&orders)
	assert.Nil(t, err)
	return orders
}

func extractOrder(t *testing.T, code int, recorder *httptest.ResponseRecorder) *models.Order {
	assert.Equal(t, code, recorder.Code)
	order := new(models.Order)
	err := json.NewDecoder(recorder.Body).Decode(order)
	assert.Nil(t, err)
	return order
}

func testContext(token *jwt.Token) context.Context {
	ctx := WithConfig(context.Background(), config)
	ctx = WithLogger(ctx, testLogger)
	return WithToken(ctx, token)
}

// -------------------------------------------------------------------------------------------------------------------
// VALIDATORS
// -------------------------------------------------------------------------------------------------------------------

func runUpdate(t *testing.T, order *models.Order, params *OrderParams) *httptest.ResponseRecorder {
	config.JWT.AdminGroupName = "admin"
	ctx := testContext(token("admin-yo", "admin@wayneindustries.com", &[]string{"admin"}))
	ctx = kami.SetParam(ctx, "id", order.ID)

	updateBody, err := json.Marshal(params)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Failed to setup test "+err.Error())
	}

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", fmt.Sprintf("https://not-real/%s", order.ID), bytes.NewReader(updateBody))

	api := NewAPI(config, db, nil)
	api.OrderUpdate(ctx, recorder, req)
	return recorder
}

func validateError(t *testing.T, code int, body *bytes.Buffer) {
	a := assert.New(t)

	errRsp := make(map[string]interface{})
	err := json.NewDecoder(body).Decode(&errRsp)
	a.Nil(err)

	errcode, exists := errRsp["code"]
	a.True(exists)
	a.EqualValues(code, errcode)

	_, exists = errRsp["msg"]
	a.True(exists)
}

func validateOrder(t *testing.T, expected, actual *models.Order) {
	a := assert.New(t)

	// all the stock fields
	a.Equal(expected.ID, actual.ID)
	a.Equal(expected.UserID, actual.UserID)
	a.Equal(expected.Email, actual.Email)
	a.Equal(expected.Currency, actual.Currency)
	a.Equal(expected.Taxes, actual.Taxes)
	a.Equal(expected.Shipping, actual.Shipping)
	a.Equal(expected.SubTotal, actual.SubTotal)
	a.Equal(expected.Total, actual.Total)
	a.Equal(expected.PaymentState, actual.PaymentState)
	a.Equal(expected.FulfillmentState, actual.FulfillmentState)
	a.Equal(expected.State, actual.State)
	a.Equal(expected.ShippingAddressID, actual.ShippingAddressID)
	a.Equal(expected.BillingAddressID, actual.BillingAddressID)
	a.Equal(expected.CreatedAt.Unix(), actual.CreatedAt.Unix())
	a.Equal(expected.UpdatedAt.Unix(), actual.UpdatedAt.Unix())
	a.Equal(expected.VATNumber, actual.VATNumber)

	// we don't return the actual user
	a.Nil(actual.User)

	for _, exp := range expected.LineItems {
		found := false
		for _, act := range expected.LineItems {
			if act.ID == exp.ID {
				found = true
				a.Equal(exp, act)
			}
		}
		a.True(found, fmt.Sprintf("Failed to find line item: %d", exp.ID))
	}
}

func validateAddress(t *testing.T, expected models.Address, actual models.Address) {
	a := assert.New(t)
	a.Equal(expected.FirstName, actual.FirstName)
	a.Equal(expected.LastName, actual.LastName)
	a.Equal(expected.Company, actual.Company)
	a.Equal(expected.Address1, actual.Address1)
	a.Equal(expected.Address2, actual.Address2)
	a.Equal(expected.City, actual.City)
	a.Equal(expected.Country, actual.Country)
	a.Equal(expected.State, actual.State)
	a.Equal(expected.Zip, actual.Zip)
}

func validateLineItem(t *testing.T, expected *models.LineItem, actual *models.LineItem) {
	a := assert.New(t)

	a.Equal(expected.ID, actual.ID)
	a.Equal(expected.Title, actual.Title)
	a.Equal(expected.SKU, actual.SKU)
	a.Equal(expected.Type, actual.Type)
	a.Equal(expected.Description, actual.Description)
	a.Equal(expected.VAT, actual.VAT)
	a.Equal(expected.Path, actual.Path)
	a.Equal(expected.Price, actual.Price)
	a.Equal(expected.Quantity, actual.Quantity)
}

func validateNewUserEmail(t *testing.T, order *models.Order, claims *JWTClaims, expectedUserEmail, expectedOrderEmail string) {
	a := assert.New(t)
	result := db.First(new(models.User), "id = ?", claims.ID)
	if !result.RecordNotFound() {
		a.FailNow("Unclean test env -- user exists with ID " + claims.ID)
	}

	err := setOrderEmail(db, order, claims, testLogger)
	if a.NoError(err) {
		user := new(models.User)
		result = db.First(user, "id = ?", claims.ID)
		a.False(result.RecordNotFound())
		a.Equal(claims.ID, user.ID)
		a.Equal(claims.ID, order.UserID)
		a.Equal(expectedOrderEmail, order.Email)
		a.Equal(expectedUserEmail, user.Email)

		db.Unscoped().Delete(user)
		t.Logf("Deleted user %s", claims.ID)
	}
}

func validateExistingUserEmail(t *testing.T, order *models.Order, claims *JWTClaims, expectedOrderEmail string) {
	a := assert.New(t)
	err := setOrderEmail(db, order, claims, testLogger)
	if a.NoError(err) {
		a.Equal(claims.ID, order.UserID)
		a.Equal(expectedOrderEmail, order.Email)
	}
}

func getTestAddress() *models.Address {
	return &models.Address{
		LastName:  "parker",
		FirstName: "Peter",
		Address1:  "123 spidey lane",
		Country:   "marvel-land",
		City:      "new york",
		Zip:       "10007",
	}
}
