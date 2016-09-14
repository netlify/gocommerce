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
	assert := assert.New(t)

	ctx := testContext(token(testUser.ID, testUser.Email, nil))
	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "https://not-real", nil)

	api := NewAPI(config, db, nil)
	api.OrderList(ctx, recorder, req)

	orders := extractOrders(t, 200, recorder)
	assert.Equal(2, len(orders))

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
			assert.Fail(fmt.Sprintf("unexpected order: %+v\n", o))
		}
	}
}

func TestQueryForAllOrdersAsAdmin(t *testing.T) {
	assert := assert.New(t)

	config.JWT.AdminGroupName = "admin"
	ctx := testContext(token("admin-yo", "admin@wayneindustries.com", &[]string{"admin"}))
	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", urlWithUserID, nil)

	api := NewAPI(config, db, nil)
	api.OrderList(ctx, recorder, req)
	orders := extractOrders(t, 200, recorder)

	assert.Equal(2, len(orders))
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
			assert.Fail(fmt.Sprintf("unexpected order: %+v\n", o))
		}
	}
}

func TestQueryForAllOrdersAsStranger(t *testing.T) {
	assert := assert.New(t)

	ctx := testContext(token("stranger", "stranger-danger@wayneindustries.com", nil))
	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "https://not-real", nil)

	api := NewAPI(config, db, nil)
	api.OrderList(ctx, recorder, req)
	assert.Equal(200, recorder.Code)
	assert.Equal("[]\n", recorder.Body.String())
}

func TestQueryForAllOrdersNotWithAdminRights(t *testing.T) {
	assert := assert.New(t)
	ctx := testContext(token("stranger", "stranger-danger@wayneindustries.com", nil))

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", urlWithUserID, nil)

	api := NewAPI(config, db, nil)
	api.OrderList(ctx, recorder, req)
	assert.Equal(400, recorder.Code)
	validateError(t, 400, recorder.Body)
}

func TestQueryForAllOrdersWithNoToken(t *testing.T) {
	assert := assert.New(t)
	ctx := testContext(nil)

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", urlWithUserID, nil)

	api := NewAPI(config, nil, nil)
	api.OrderList(ctx, recorder, req)
	assert.Equal(401, recorder.Code)
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
	assert := assert.New(t)
	ctx := testContext(token("stranger", "stranger-danger@wayneindustries.com", nil))

	// have to add it to the context ~ it isn't from the params
	ctx = kami.SetParam(ctx, "id", firstOrder.ID)

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", urlForFirstOrder, nil)

	api := NewAPI(config, db, nil)
	api.OrderView(ctx, recorder, req)
	assert.Equal(401, recorder.Code)
	validateError(t, 401, recorder.Body)
}

func TestQueryForAMissingOrder(t *testing.T) {
	assert := assert.New(t)
	ctx := testContext(token("stranger", "stranger-danger@wayneindustries.com", nil))

	// have to add it to the context ~ it isn't from the params
	ctx = kami.SetParam(ctx, "id", "does-not-exist")

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "https://not-real/does-not-exist", nil)

	api := NewAPI(config, db, nil)
	api.OrderView(ctx, recorder, req)
	assert.Equal(404, recorder.Code)
	validateError(t, 404, recorder.Body)
}

func TestQueryForAnOrderWithNoToken(t *testing.T) {
	assert := assert.New(t)
	ctx := testContext(nil)

	// have to add it to the context ~ it isn't from the params
	ctx = kami.SetParam(ctx, "id", "does-not-exist")

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "https://not-real/does-not-exist", nil)

	// use nil for DB b/c it should *NEVER* be called
	api := NewAPI(config, nil, nil)
	api.OrderView(ctx, recorder, req)
	assert.Equal(401, recorder.Code)
	validateError(t, 401, recorder.Body)
}

// -------------------------------------------------------------------------------------------------------------------
// CREATE
// -------------------------------------------------------------------------------------------------------------------
// TODO vvvv ~ need to make it verifiable
//func TestCreateAnOrderAsAnExistingUser(t *testing.T) {
//	assert := assert.New(t)
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
//	assert.Equal(200, recorder.Code)
//
//	//ret := new(models.Order)
//	ret := make(map[string]interface{})
//	err = json.Unmarshal(recorder.Body.Bytes(), ret)
//	assert.NoError(err)
//
//	fmt.Printf("%+v\n", ret)
//}

// --------------------------------------------------------------------------------------------------------------------
// Create ~ email logic
// --------------------------------------------------------------------------------------------------------------------
func TestSetUserIDLogic_AnonymousUser(t *testing.T) {
	assert := assert.New(t)
	simpleOrder := models.NewOrder("session", "params@email.com", "usd")
	err := setOrderEmail(nil, simpleOrder, nil, testLogger)
	assert.Nil(err)
	assert.Equal("params@email.com", simpleOrder.Email)
}

func TestSetUserIDLogic_AnonymousUserWithNoEmail(t *testing.T) {
	assert := assert.New(t)
	simpleOrder := models.NewOrder("session", "", "usd")
	err := setOrderEmail(nil, simpleOrder, nil, testLogger)
	if !assert.Error(err) {
		assert.Equal(400, err.Code)
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
	assert := assert.New(t)
	simpleOrder := models.NewOrder("session", "", "usd")
	claims := token("alfred", "", nil).Claims.(*JWTClaims)
	err := setOrderEmail(db, simpleOrder, claims, testLogger)
	if assert.Error(err) {
		assert.Equal(400, err.Code)
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
	assert := assert.New(t)

	recorder := runUpdate(t, firstOrder, &OrderParams{
		Email:    "mrfreeze@dc.com",
		Currency: "monopoly-dollars",
	})
	rspOrder := extractOrder(t, 200, recorder)

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

func TestUpdateAddress_ExistingAddress(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	defer db.Save(firstOrder)
	assert := assert.New(t)

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
	assert.False(rsp.RecordNotFound())

	// now we load the addresses
	assert.Equal(saved.BillingAddressID, rspOrder.BillingAddressID)

	savedAddr := &models.Address{ID: saved.BillingAddressID}
	rsp = db.First(savedAddr)
	assert.False(rsp.RecordNotFound())
	defer db.Unscoped().Delete(savedAddr)

	validateAddress(t, *newAddr, *savedAddr)
}

func TestUpdateAddress_NewAddress(t *testing.T) {
	defer db.Save(firstOrder)
	assert := assert.New(t)

	paramsAddress := getTestAddress()
	recorder := runUpdate(t, firstOrder, &OrderParams{
		// should create a new address associated with the order's user
		ShippingAddress: paramsAddress,
	})
	rspOrder := extractOrder(t, 200, recorder)

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

func TestUpdateWithNewData(t *testing.T) {
	assert := assert.New(t)
	defer db.Where("order_id = ?", firstOrder.ID).Delete(&models.Data{})

	op := &OrderParams{
		Data: map[string]interface{}{
			"thing":       1,
			"red":         "fish",
			"other thing": 3.4,
			"exists":      true,
		},
	}
	recorder := runUpdate(t, firstOrder, op)
	returnedOrder := extractOrder(t, 200, recorder)

	// TODO test that the returned order contains the data we expect
	_ = returnedOrder

	datas := []models.Data{}
	db.Where("order_id = ?", firstOrder.ID).Find(&datas)
	assert.Len(datas, 4)
	for _, datum := range datas {
		switch datum.Key {
		case "thing":
			assert.Equal(models.NumberType, datum.Type)
			assert.EqualValues(1, datum.NumericValue)
		case "red":
			assert.Equal(models.StringType, datum.Type)
			assert.Equal("fish", datum.StringValue)
		case "other thing":
			assert.Equal(models.NumberType, datum.Type)
			assert.EqualValues(3.4, datum.NumericValue)
		case "exists":
			assert.Equal(models.BoolType, datum.Type)
			assert.Equal(true, datum.BoolValue)
		}
	}
}

func TestUpdateWithBadData(t *testing.T) {
	defer db.Where("order_id = ?", firstOrder.ID).Delete(&models.Data{})

	op := &OrderParams{
		Data: map[string]interface{}{
			"array": []int{4},
		},
	}
	recorder := runUpdate(t, firstOrder, op)
	validateError(t, 400, recorder.Body)
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
	assert := assert.New(t)

	errRsp := make(map[string]interface{})
	err := json.NewDecoder(body).Decode(&errRsp)
	assert.Nil(err)

	errcode, exists := errRsp["code"]
	assert.True(exists)
	assert.EqualValues(code, errcode)

	_, exists = errRsp["msg"]
	assert.True(exists)
}

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

func validateAddress(t *testing.T, expected models.Address, actual models.Address) {
	assert := assert.New(t)
	assert.Equal(expected.FirstName, actual.FirstName)
	assert.Equal(expected.LastName, actual.LastName)
	assert.Equal(expected.Company, actual.Company)
	assert.Equal(expected.Address1, actual.Address1)
	assert.Equal(expected.Address2, actual.Address2)
	assert.Equal(expected.City, actual.City)
	assert.Equal(expected.Country, actual.Country)
	assert.Equal(expected.State, actual.State)
	assert.Equal(expected.Zip, actual.Zip)
}

func validateLineItem(t *testing.T, expected *models.LineItem, actual *models.LineItem) {
	assert := assert.New(t)

	assert.Equal(expected.ID, actual.ID)
	assert.Equal(expected.Title, actual.Title)
	assert.Equal(expected.SKU, actual.SKU)
	assert.Equal(expected.Type, actual.Type)
	assert.Equal(expected.Description, actual.Description)
	assert.Equal(expected.VAT, actual.VAT)
	assert.Equal(expected.Path, actual.Path)
	assert.Equal(expected.Price, actual.Price)
	assert.Equal(expected.Quantity, actual.Quantity)
}

func validateNewUserEmail(t *testing.T, order *models.Order, claims *JWTClaims, expectedUserEmail, expectedOrderEmail string) {
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

func validateExistingUserEmail(t *testing.T, order *models.Order, claims *JWTClaims, expectedOrderEmail string) {
	assert := assert.New(t)
	err := setOrderEmail(db, order, claims, testLogger)
	if assert.NoError(err) {
		assert.Equal(claims.ID, order.UserID)
		assert.Equal(expectedOrderEmail, order.Email)
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
