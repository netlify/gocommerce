package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/stretchr/testify/assert"

	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/models"
)

var testLogger = logrus.NewEntry(logrus.StandardLogger())
var config = &conf.Configuration{}
var db *gorm.DB

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

	os.Exit(m.Run())
}

func TestQueryForOrdersAsTheUser(t *testing.T) {
	a := assert.New(t)
	token := token(testUser.ID, testUser.Email, nil)
	ctx := &RequestContext{}
	ctx = ctx.WithConfig(config).WithLogger(testLogger).WithToken(token)

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "https://not-real", nil)

	api := NewAPI(config, db, nil)
	api.OrderList(*ctx, recorder, req)
	a.Equal(200, recorder.Code)

	orders := []models.Order{}
	err := json.NewDecoder(recorder.Body).Decode(&orders)
	a.Nil(err)
	a.Equal(2, len(orders))

	for _, o := range orders {
		switch o.ID {
		case firstOrder.ID:
			validateOrder(t, firstOrder, &o)
		case secondOrder.ID:
			validateOrder(t, secondOrder, &o)
		default:
			a.Fail(fmt.Sprintf("unexpected order: %+v\n", o))
		}
	}
}

func TestQueryForOrdersAsAdmin(t *testing.T) {
	a := assert.New(t)
	config.JWT.AdminGroupName = "admin"
	token := token("admin-yo", "admin@wayneindustries.com", &[]string{"admin"})
	ctx := new(RequestContext).WithConfig(config).WithLogger(testLogger).WithToken(token)

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("https://not-real?user_id=%s", testUser.ID), nil)

	api := NewAPI(config, db, nil)
	api.OrderList(*ctx, recorder, req)
	a.Equal(200, recorder.Code)

	orders := []models.Order{}
	err := json.NewDecoder(recorder.Body).Decode(&orders)
	a.Nil(err)
	a.Equal(2, len(orders))

	for _, o := range orders {
		switch o.ID {
		case firstOrder.ID:
			validateOrder(t, firstOrder, &o)
		case secondOrder.ID:
			validateOrder(t, secondOrder, &o)
		default:
			a.Fail(fmt.Sprintf("unexpected order: %+v\n", o))
		}
	}
}

func TestQueryForOrdersAsStranger(t *testing.T) {
	a := assert.New(t)
	token := token("stranger", "stranger-danger@wayneindustries.com", nil)
	ctx := new(RequestContext).WithConfig(config).WithLogger(testLogger).WithToken(token)

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "https://not-real", nil)

	api := NewAPI(config, db, nil)
	api.OrderList(*ctx, recorder, req)
	a.Equal(200, recorder.Code)
	a.Equal("[]\n", recorder.Body.String())
}

func TestQueryForOrderNotWithAdminRights(t *testing.T) {
	a := assert.New(t)
	token := token("stranger", "stranger-danger@wayneindustries.com", nil)
	ctx := new(RequestContext).WithConfig(config).WithLogger(testLogger).WithToken(token)

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("https://not-real?user_id=%s", testUser.ID), nil)

	api := NewAPI(config, db, nil)
	api.OrderList(*ctx, recorder, req)

	errRsp := make(map[string]interface{})
	err := json.NewDecoder(recorder.Body).Decode(&errRsp)
	a.Nil(err)

	code, exists := errRsp["code"]
	a.True(exists)
	a.EqualValues(400, code)

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
	validateAddress(t, expected.BillingAddress, actual.BillingAddress)
	validateAddress(t, expected.ShippingAddress, actual.ShippingAddress)
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
