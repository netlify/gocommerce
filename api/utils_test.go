package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"

	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/models"
	tu "github.com/netlify/gocommerce/testutils"
)

var testLogger = logrus.NewEntry(logrus.StandardLogger())
var db *gorm.DB

var urlWithUserID string
var urlForFirstOrder string

func TestMain(m *testing.M) {
	f, err := ioutil.TempFile("", "test-db")
	if err != nil {
		panic(err)
	}
	defer os.Remove(f.Name())

	config := testConfig()
	config.DB.Driver = "sqlite3"
	config.DB.ConnURL = f.Name()

	// setup test db
	db, err = models.Connect(config)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	tu.LoadTestData(db)
	urlForFirstOrder = fmt.Sprintf("https://not-real/%s", tu.FirstOrder.ID)
	urlWithUserID = fmt.Sprintf("https://not-real?user_id=%s", tu.TestUser.ID)

	os.Exit(m.Run())
}

func testContext(token *jwt.Token, config *conf.Configuration) context.Context {
	ctx := WithConfig(context.Background(), config)
	ctx = WithLogger(ctx, testLogger)
	return WithToken(ctx, token)
}

func testConfig() *conf.Configuration {
	return &conf.Configuration{}
}

func token(id, email string, groups *[]string) *jwt.Token {
	claims := &JWTClaims{
		ID:     id,
		Email:  email,
		Groups: []string{},
	}
	if groups != nil {
		for _, g := range *groups {
			claims.Groups = append(claims.Groups, g)
		}
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t
}

// ------------------------------------------------------------------------------------------------
// validators
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
// extractors
// ------------------------------------------------------------------------------------------------

func extractPayload(t *testing.T, code int, recorder *httptest.ResponseRecorder, what interface{}) {
	if recorder.Code == code {
		err := json.NewDecoder(recorder.Body).Decode(what)
		if !assert.NoError(t, err) {
			assert.FailNow(t, "Failed to extract body")
		}
	} else {
		assert.FailNow(t, "Unexpected code: ", code)
	}
}
