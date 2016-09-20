package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
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

func testConfig() *conf.Configuration {
	return &conf.Configuration{}
}
