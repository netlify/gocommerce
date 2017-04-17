package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"

	"github.com/netlify/gocommerce/calculator"
	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/models"
)

var dbFiles []string
var testLogger = logrus.NewEntry(logrus.StandardLogger())

var urlWithUserID string
var urlForFirstOrder string

func TestMain(m *testing.M) {
	dbFiles = []string{}
	defer func() {
		fmt.Printf("removing lingering %d db files\n", len(dbFiles))
		for _, f := range dbFiles {
			os.Remove(f)
		}
	}()

	os.Exit(m.Run())
}

func db(t *testing.T) (*gorm.DB, *conf.Configuration) {
	f, err := ioutil.TempFile("", "test-db")
	if err != nil {
		panic(err)
	}
	dbFiles = append(dbFiles, f.Name())

	config := testConfig()
	config.DB.Driver = "sqlite3"
	config.DB.ConnURL = f.Name()

	db, err := models.Connect(config)
	if err != nil {
		assert.FailNow(t, "failed to connect to db: "+err.Error())
	}

	loadTestData(db)
	urlForFirstOrder = fmt.Sprintf("https://not-real/%s", firstOrder.ID)
	urlWithUserID = fmt.Sprintf("https://not-real/users/%s/orders", testUser.ID)

	return db, config
}

func testContext(token *jwt.Token, config *conf.Configuration, adminFlag bool) context.Context {
	ctx := context.Background()
	ctx = withConfig(ctx, config)
	ctx = withRequestID(ctx, "test-request")
	ctx = withLogger(ctx, testLogger)
	ctx = withStartTime(ctx, time.Now())
	ctx = withAdminFlag(ctx, adminFlag)
	return withToken(ctx, token)
}

func testConfig() *conf.Configuration {
	config := new(conf.Configuration)
	config.DB.Automigrate = true
	return config
}

func testToken(id, email string) *jwt.Token {
	claims := &JWTClaims{
		ID:    id,
		Email: email,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t
}

// ------------------------------------------------------------------------------------------------
// TEST DATA
// ------------------------------------------------------------------------------------------------
var testUser models.User
var testAddress models.Address

var firstOrder *models.Order
var firstTransaction *models.Transaction
var firstLineItem models.LineItem

var secondOrder *models.Order
var secondTransaction *models.Transaction
var secondLineItem1 models.LineItem
var secondLineItem2 models.LineItem

func loadTestData(db *gorm.DB) {
	testUser = models.User{
		ID:    "i-am-batman",
		Email: "bruce@wayneindustries.com",
	}

	testAddress = models.Address{
		AddressRequest: models.AddressRequest{
			LastName: "wayne",
			Address1: "123 cave way",
			Country:  "dcland",
			City:     "gotham",
			Zip:      "324234",
		},
		ID:   "first-address",
		User: &testUser,
	}

	firstOrder = models.NewOrder("session1", testUser.Email, "usd")
	firstOrder.UserID = testUser.ID
	firstTransaction = models.NewTransaction(firstOrder)
	firstTransaction.ProcessorID = "stripe"
	firstTransaction.Amount = 100
	firstTransaction.Status = models.PaidState
	firstLineItem = models.LineItem{
		ID:          11,
		OrderID:     firstOrder.ID,
		Title:       "batwing",
		Sku:         "123-i-can-fly-456",
		Type:        "plane",
		Description: "it's the batwing.",
		Price:       12,
		Quantity:    2,
		Path:        "/i/believe/i/can/fly",
	}

	secondOrder = models.NewOrder("session2", testUser.Email, "usd")
	secondOrder.UserID = testUser.ID
	secondTransaction = models.NewTransaction(secondOrder)
	secondLineItem1 = models.LineItem{
		ID:          21,
		OrderID:     secondOrder.ID,
		Title:       "tumbler",
		Sku:         "456-i-rollover-all-things",
		Type:        "tank",
		Description: "OMG yes",
		Price:       5,
		Quantity:    2,
		Path:        "/i/crush/villians/dreams",
	}
	secondLineItem2 = models.LineItem{
		ID:          22,
		OrderID:     secondOrder.ID,
		Title:       "utility belt",
		Sku:         "234-fancy-belts",
		Type:        "clothes",
		Description: "stlyish but still useful",
		Price:       45,
		Quantity:    1,
		Path:        "/i/hold/the/universe/on/my/waist",
	}

	db.Create(&testUser)
	db.Create(&testAddress)

	firstOrder.ID = "first-order"
	firstOrder.LineItems = []*models.LineItem{&firstLineItem}
	firstOrder.CalculateTotal(&calculator.Settings{}, nil)
	firstOrder.BillingAddress = testAddress
	firstOrder.ShippingAddress = testAddress
	firstOrder.User = &testUser
	db.Create(&firstLineItem)
	db.Create(firstTransaction)
	db.Create(firstOrder)

	secondOrder.ID = "second-order"
	secondOrder.LineItems = []*models.LineItem{&secondLineItem1, &secondLineItem2}
	secondOrder.CalculateTotal(&calculator.Settings{}, nil)
	secondOrder.BillingAddress = testAddress
	secondOrder.ShippingAddress = testAddress
	secondOrder.User = &testUser
	db.Create(&secondLineItem1)
	db.Create(&secondLineItem2)
	db.Create(secondTransaction)
	db.Create(secondOrder)
}

func getTestAddress() *models.Address {
	return &models.Address{
		ID: "spidermans-house",
		AddressRequest: models.AddressRequest{
			LastName:  "parker",
			FirstName: "Peter",
			Address1:  "123 spidey lane",
			Country:   "marvel-land",
			City:      "new york",
			Zip:       "10007",
		},
	}
}

// ------------------------------------------------------------------------------------------------
// VALIDATORS
// ------------------------------------------------------------------------------------------------

func validateError(t *testing.T, code int, recorder *httptest.ResponseRecorder) {
	assert := assert.New(t)
	if code != recorder.Code {
		assert.Fail(fmt.Sprintf("code mismatch: expected %d vs actual %d", code, recorder.Code))
		return
	}

	errRsp := make(map[string]interface{})
	err := json.NewDecoder(recorder.Body).Decode(&errRsp)
	assert.Nil(err)

	errcode, exists := errRsp["code"]
	assert.True(exists)
	assert.EqualValues(code, errcode)

	_, exists = errRsp["msg"]
	assert.True(exists)
}

func validateUser(t *testing.T, expected *models.User, actual *models.User) {
	assert := assert.New(t)
	assert.Equal(expected.ID, actual.ID)
	assert.Equal(expected.Email, actual.Email)
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

// ------------------------------------------------------------------------------------------------
// HELPERS
// ------------------------------------------------------------------------------------------------
func extractPayload(t *testing.T, code int, recorder *httptest.ResponseRecorder, what interface{}) {
	if recorder.Code == code {
		err := json.NewDecoder(recorder.Body).Decode(what)
		if !assert.NoError(t, err) {
			assert.FailNow(t, "Failed to extract body")
		}
	} else {
		assert.FailNow(t, fmt.Sprintf("Unexpected code: %v - expected: %v", recorder.Code, code))
	}
}
