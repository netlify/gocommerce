package api

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"

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

	config.DB = conf.DBConfig{
		Driver:  "sqlite3",
		ConnURL: f.Name(),
	}

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
	token := token(testUser.ID, testUser.Email, nil)
	ctx := &RequestContext{}
	ctx = ctx.WithConfig(config).WithLogger(testLogger).WithToken(token)

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "https://not-real", nil)

	api := NewAPI(config, db, nil)
	api.OrderList(*ctx, recorder, req)

	fmt.Printf("%d - %s\n", recorder.Code, recorder.Body.String())
}

func TestQueryForOrdersAsAdmin(t *testing.T) {
}

func TestQueryForOrdersAsStranger(t *testing.T) {
}
