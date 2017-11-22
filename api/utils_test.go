package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netlify/gocommerce/calculator"
	"github.com/netlify/gocommerce/claims"
	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/models"
)

const baseURL = "https://example.com"

var dbFiles []string
var testLogger = logrus.NewEntry(logrus.StandardLogger())

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

func db(t *testing.T) (*gorm.DB, *conf.GlobalConfiguration, *conf.Configuration, *TestData) {
	f, err := ioutil.TempFile("", "test-db")
	if err != nil {
		panic(err)
	}
	dbFiles = append(dbFiles, f.Name())

	globalConfig, config := testConfig()
	globalConfig.DB.Driver = "sqlite3"
	globalConfig.DB.URL = f.Name()

	db, err := models.Connect(globalConfig)
	if err != nil {
		assert.FailNow(t, "failed to connect to db: "+err.Error())
	}

	data := loadTestData(t, db)
	return db, globalConfig, config, data
}

func testConfig() (*conf.GlobalConfiguration, *conf.Configuration) {
	logrus.SetLevel(logrus.ErrorLevel)

	globalConfig := new(conf.GlobalConfiguration)
	globalConfig.DB.Automigrate = true

	config := new(conf.Configuration)
	config.JWT.Secret = "testsecret"
	config.JWT.AdminGroupName = "admin"
	config.Payment.Stripe.Enabled = true
	config.Payment.Stripe.SecretKey = "secret"
	return globalConfig, config
}

func testToken(id, email string) *jwt.Token {
	claims := &claims.JWTClaims{
		StandardClaims: jwt.StandardClaims{
			Subject: id,
		},
		Email: email,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
}

func testExpiredToken(id, email string) *jwt.Token {
	claims := &claims.JWTClaims{
		StandardClaims: jwt.StandardClaims{
			Subject:   id,
			ExpiresAt: time.Now().Add(time.Duration(-1) * time.Minute).Unix(),
		},
		Email: email,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
}

func testAdminToken(id, email string) *jwt.Token {
	claims := &claims.JWTClaims{
		StandardClaims: jwt.StandardClaims{
			Subject: id,
		},
		Email: email,
		AppMetaData: map[string]interface{}{
			"roles": []interface{}{"admin"},
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
}

// ------------------------------------------------------------------------------------------------
// TEST DATA
// ------------------------------------------------------------------------------------------------

type TestData struct {
	urlWithUserID    string
	urlForFirstOrder string

	testUserToken *jwt.Token
	testUser      *models.User
	testAddress   models.Address

	firstOrder       *models.Order
	firstTransaction *models.Transaction
	firstLineItem    *models.LineItem

	secondOrder       *models.Order
	secondTransaction *models.Transaction
	secondLineItem1   *models.LineItem
	secondLineItem2   *models.LineItem
}

func setupTestData() *TestData {
	testUser := &models.User{
		ID:    "i-am-batman",
		Email: "bruce@wayneindustries.com",
	}

	testAddress := models.Address{
		ID: "first-address",
		AddressRequest: models.AddressRequest{
			Name:     "wayne",
			Address1: "123 cave way",
			Country:  "dcland",
			City:     "gotham",
			Zip:      "324234",
		},
		User: testUser,
	}

	firstOrder := models.NewOrder("", "session1", testUser.Email, "USD")
	firstOrder.UserID = testUser.ID
	firstOrder.PaymentProcessor = "stripe"
	firstOrder.PaymentState = models.PaidState
	firstTransaction := models.NewTransaction(firstOrder)
	firstTransaction.ProcessorID = "stripe"
	firstTransaction.Amount = 100
	firstTransaction.Status = models.PaidState
	firstLineItem := &models.LineItem{
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

	firstDownload := models.Download{
		Title: firstLineItem.Title,
		Sku:   firstLineItem.Sku,
		ID:    "first-download",
	}

	firstOrder.ID = "first-order"
	firstOrder.LineItems = []*models.LineItem{firstLineItem}
	firstOrder.Downloads = []models.Download{firstDownload}
	firstOrder.CalculateTotal(&calculator.Settings{}, nil, testLogger)
	firstOrder.BillingAddress = testAddress
	firstOrder.ShippingAddress = testAddress
	firstOrder.User = testUser
	firstOrder.CouponCode = "zerodiscount"
	firstTransaction.ID = "first-trans"

	secondOrder := models.NewOrder("", "session2", testUser.Email, "USD")
	secondOrder.UserID = testUser.ID
	secondOrder.PaymentProcessor = "paypal"
	secondOrder.PaymentState = models.PaidState
	secondTransaction := models.NewTransaction(secondOrder)
	secondTransaction.ProcessorID = "paypal"
	secondLineItem1 := &models.LineItem{
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
	secondLineItem2 := &models.LineItem{
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

	secondOrder.ID = "second-order"
	secondOrder.LineItems = []*models.LineItem{secondLineItem1, secondLineItem2}
	secondOrder.CalculateTotal(&calculator.Settings{}, nil, testLogger)
	secondOrder.BillingAddress = testAddress
	secondOrder.ShippingAddress = testAddress
	secondOrder.User = testUser
	secondTransaction.ID = "second-trans"
	secondTransaction.Amount = secondOrder.Total
	secondTransaction.Status = models.PaidState

	return &TestData{
		fmt.Sprintf("/users/%s/orders", testUser.ID),
		fmt.Sprintf("/orders/%s", firstOrder.ID),

		testToken(testUser.ID, testUser.Email),
		testUser,
		testAddress,

		firstOrder,
		firstTransaction,
		firstLineItem,

		secondOrder,
		secondTransaction,
		secondLineItem1,
		secondLineItem2,
	}
}

func loadTestData(t *testing.T, db *gorm.DB) *TestData {
	testData := setupTestData()

	require.NoError(t, db.Create(testData.testUser).Error)
	require.NoError(t, db.Create(&testData.testAddress).Error)

	require.NoError(t, db.Create(testData.firstLineItem).Error)
	require.NoError(t, db.Create(testData.firstOrder).Error)
	require.NoError(t, db.Create(testData.firstTransaction).Error)
	for _, d := range testData.firstOrder.Downloads {
		require.NoError(t, db.Create(d).Error)
	}

	require.NoError(t, db.Create(testData.secondLineItem1).Error)
	require.NoError(t, db.Create(testData.secondLineItem2).Error)
	require.NoError(t, db.Create(testData.secondOrder).Error)
	require.NoError(t, db.Create(testData.secondTransaction).Error)
	return testData
}

func getTestAddress() *models.Address {
	return &models.Address{
		ID: "spidermans-house",
		AddressRequest: models.AddressRequest{
			Name:     "Peter Parker",
			Address1: "123 spidey lane",
			Country:  "marvel-land",
			City:     "new york",
			Zip:      "10007",
		},
	}
}

// ------------------------------------------------------------------------------------------------
// VALIDATORS
// ------------------------------------------------------------------------------------------------

func validateError(t *testing.T, code int, recorder *httptest.ResponseRecorder, msgs ...string) {
	assert := assert.New(t)
	require.Equal(t, code, recorder.Code, "code mismatch: %v", recorder.Body)

	errRsp := make(map[string]interface{})
	err := json.NewDecoder(recorder.Body).Decode(&errRsp)
	assert.Nil(err)

	errcode, exists := errRsp["code"]
	assert.True(exists)
	assert.EqualValues(code, errcode)

	msg, exists := errRsp["msg"]
	assert.True(exists)

	for _, m := range msgs {
		assert.Contains(msg, m, "msg must contain")
	}
}

func validateUser(t *testing.T, expected *models.User, actual *models.User) {
	assert := assert.New(t)
	assert.Equal(expected.ID, actual.ID)
	assert.Equal(expected.Email, actual.Email)
}

func validateAddress(t *testing.T, expected models.Address, actual models.Address) {
	assert := assert.New(t)
	assert.Equal(expected.Name, actual.Name)
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
	require.Equal(t, code, recorder.Code, "code mismatch: %v", recorder.Body)

	err := json.NewDecoder(recorder.Body).Decode(what)
	require.NoError(t, err, "Failed to extract body: %s", string(recorder.Body.Bytes()))
}

type RouteTest struct {
	DB           *gorm.DB
	GlobalConfig *conf.GlobalConfiguration
	Config       *conf.Configuration
	T            *testing.T
	Data         *TestData
}

func NewRouteTest(t *testing.T) *RouteTest {
	db, globalConfig, config, data := db(t)
	return &RouteTest{db, globalConfig, config, t, data}
}

func (r *RouteTest) TestEndpoint(method string, url string, body io.Reader, token *jwt.Token) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, baseURL+url, body)

	if token != nil {
		require.NoError(r.T, signHTTPRequest(req, token, r.Config.JWT.Secret))
	}
	globalConfig := new(conf.GlobalConfiguration)
	ctx, err := WithInstanceConfig(context.Background(), globalConfig.SMTP, r.Config, "")
	require.NoError(r.T, err)
	NewAPIWithVersion(ctx, r.GlobalConfig, r.DB, "").handler.ServeHTTP(recorder, req)

	return recorder
}

func signHTTPRequest(req *http.Request, token *jwt.Token, jwtSecret string) error {
	tokenStr, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))
	return nil
}
