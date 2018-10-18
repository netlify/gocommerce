package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/netlify/gocommerce/calculator"
	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/models"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type MiddlewareTestSuite struct {
	suite.Suite
	API         *API
	exampleSite *httptest.Server
}

func (ts *MiddlewareTestSuite) SetupTest() {
	globalConfig, err := conf.LoadGlobal("test.env")
	require.NoError(ts.T(), err)
	globalConfig.MultiInstanceMode = true
	db, err := models.Connect(globalConfig)
	require.NoError(ts.T(), err)

	api := NewAPI(globalConfig, db)
	ts.API = api
}

func (ts *MiddlewareTestSuite) TearDownTest() {
	if ts.exampleSite != nil {
		ts.exampleSite.Close()
		ts.exampleSite = nil
	}

	// Cleanup created instance
	i, err := models.GetInstanceByUUID(ts.API.db, testUUID)
	if err == nil {
		require.NoError(ts.T(), models.DeleteInstance(ts.API.db, i))
	}
}

func (ts *MiddlewareTestSuite) setupInstance(config *conf.Configuration, siteSettings interface{}) string {
	if config == nil {
		_, config = testConfig()
	}

	if siteSettings == nil {
		siteSettings = struct{}{}
	}
	ts.exampleSite = startTestSiteWithSettings(siteSettings)
	config.SiteURL = ts.exampleSite.URL

	instanceID := uuid.NewRandom().String()
	err := models.CreateInstance(ts.API.db, &models.Instance{
		ID:         instanceID,
		UUID:       testUUID,
		BaseConfig: config,
	})
	require.NoError(ts.T(), err)

	return instanceID
}

func (ts *MiddlewareTestSuite) TestWithInstanceConfig() {
	instanceID := ts.setupInstance(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "http://localhost/settings", nil)
	req.Header.Set("Content-Type", "application/json")
	err := signInstanceRequest(req, instanceID, ts.API.config.OperatorToken)
	require.NoError(ts.T(), err)

	w := httptest.NewRecorder()
	ts.API.handler.ServeHTTP(w, req)
	require.Equal(ts.T(), http.StatusOK, w.Code)

	settingsPayload := &calculator.Settings{
		PricesIncludeTaxes: false,
		PaymentMethods:     &calculator.PaymentMethods{},
	}
	settingsPayload.PaymentMethods.Stripe.Enabled = true

	parsedBody := &calculator.Settings{}
	err = json.NewDecoder(w.Body).Decode(parsedBody)
	require.NoError(ts.T(), err)

	require.EqualValues(ts.T(), settingsPayload, parsedBody)
}

func (ts *MiddlewareTestSuite) TestWithInstanceConfig_NoPaymentProviders() {
	instanceID := ts.setupInstance(&conf.Configuration{}, nil)

	req := httptest.NewRequest(http.MethodGet, "http://localhost/settings", nil)
	req.Header.Set("Content-Type", "application/json")
	err := signInstanceRequest(req, instanceID, ts.API.config.OperatorToken)
	require.NoError(ts.T(), err)

	w := httptest.NewRecorder()
	ts.API.handler.ServeHTTP(w, req)
	require.Equal(ts.T(), http.StatusInternalServerError, w.Code)
}

func TestMiddleware(t *testing.T) {
	suite.Run(t, new(MiddlewareTestSuite))
}
