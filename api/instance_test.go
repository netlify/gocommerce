package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pborman/uuid"

	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const testUUID = "11111111-1111-1111-1111-111111111111"
const operatorToken = "operatorToken"

type InstanceTestSuite struct {
	suite.Suite
	API *API
}

func (ts *InstanceTestSuite) SetupTest() {
	globalConfig, err := conf.LoadGlobal("test.env")
	require.NoError(ts.T(), err)
	globalConfig.OperatorToken = operatorToken
	globalConfig.MultiInstanceMode = true
	db, err := models.Connect(globalConfig)
	require.NoError(ts.T(), err)

	api := NewAPI(globalConfig, db)
	ts.API = api

	// Cleanup existing instance
	i, err := models.GetInstanceByUUID(db, testUUID)
	if err == nil {
		require.NoError(ts.T(), models.DeleteInstance(db, i))
	}
}

func (ts *InstanceTestSuite) TestCreate() {
	// Request body
	var buffer bytes.Buffer
	require.NoError(ts.T(), json.NewEncoder(&buffer).Encode(map[string]interface{}{
		"uuid":     testUUID,
		"site_url": "https://example.netlify.com",
		"config": map[string]interface{}{
			"jwt": map[string]interface{}{
				"secret": "testsecret",
			},
		},
	}))

	// Setup request
	req := httptest.NewRequest(http.MethodPost, "http://localhost/instances", &buffer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+operatorToken)

	// Setup response recorder
	w := httptest.NewRecorder()

	ts.API.handler.ServeHTTP(w, req)
	require.Equal(ts.T(), http.StatusCreated, w.Code)
	resp := models.Instance{}
	require.NoError(ts.T(), json.NewDecoder(w.Body).Decode(&resp))
	assert.NotNil(ts.T(), resp.BaseConfig)

	i, err := models.GetInstanceByUUID(ts.API.db, testUUID)
	require.NoError(ts.T(), err)
	assert.NotNil(ts.T(), i.BaseConfig)
}

func (ts *InstanceTestSuite) TestGet() {
	instanceID := uuid.NewRandom().String()
	err := models.CreateInstance(ts.API.db, &models.Instance{
		ID:   instanceID,
		UUID: testUUID,
		BaseConfig: &conf.Configuration{
			JWT: conf.JWTConfiguration{
				Secret: "testsecret",
			},
		},
	})
	require.NoError(ts.T(), err)

	req := httptest.NewRequest(http.MethodGet, "http://localhost/instances/"+instanceID, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+operatorToken)

	w := httptest.NewRecorder()
	ts.API.handler.ServeHTTP(w, req)
	require.Equal(ts.T(), http.StatusOK, w.Code)
	resp := models.Instance{}
	require.NoError(ts.T(), json.NewDecoder(w.Body).Decode(&resp))

	assert.Equal(ts.T(), "testsecret", resp.BaseConfig.JWT.Secret)
}

func (ts *InstanceTestSuite) TestUpdate() {
	instanceID := uuid.NewRandom().String()
	err := models.CreateInstance(ts.API.db, &models.Instance{
		ID:   instanceID,
		UUID: testUUID,
		BaseConfig: &conf.Configuration{
			JWT: conf.JWTConfiguration{
				Secret: "testsecret",
			},
		},
	})
	require.NoError(ts.T(), err)

	var buffer bytes.Buffer
	require.NoError(ts.T(), json.NewEncoder(&buffer).Encode(map[string]interface{}{
		"config": &conf.Configuration{
			JWT: conf.JWTConfiguration{
				Secret: "testsecret",
			},
			SiteURL: "https://test.mysite.com",
		},
	}))

	req := httptest.NewRequest(http.MethodPut, "http://localhost/instances/"+instanceID, &buffer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+operatorToken)

	w := httptest.NewRecorder()
	ts.API.handler.ServeHTTP(w, req)
	require.Equal(ts.T(), http.StatusOK, w.Code)

	i, err := models.GetInstanceByUUID(ts.API.db, testUUID)
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), "testsecret", i.BaseConfig.JWT.Secret)
	require.Equal(ts.T(), "https://test.mysite.com", i.BaseConfig.SiteURL)
}

func TestInstance(t *testing.T) {
	suite.Run(t, new(InstanceTestSuite))
}
