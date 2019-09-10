package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/netlify/gocommerce/models"
	"github.com/stretchr/testify/assert"
)

func TestDownloadList(t *testing.T) {
	t.Run("UserList", func(t *testing.T) {
		test := NewRouteTest(t)
		token := test.Data.testUserToken
		recorder := test.TestEndpoint(http.MethodGet, "/downloads", nil, token)

		downloads := []models.Download{}
		extractPayload(t, http.StatusOK, recorder, &downloads)
		assert.Len(t, downloads, 1)
	})
}

func currentDownloads(test *RouteTest) []models.Download {
	recorder := test.TestEndpoint(http.MethodGet, "/downloads", nil, test.Data.testUserToken)

	downloads := []models.Download{}
	extractPayload(test.T, http.StatusOK, recorder, &downloads)
	return downloads
}

type DownloadMeta struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

func startTestSiteWithDownloads(t *testing.T, downloads []*DownloadMeta) *httptest.Server {
	downloadsList, err := json.Marshal(downloads)
	assert.NoError(t, err)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/i/believe/i/can/fly":
			fmt.Fprintf(w, productMetaFrame(`
				{"sku": "123-i-can-fly-456", "downloads": %s}`),
				string(downloadsList),
			)
		}
	}))
}

func TestDownloadRefresh(t *testing.T) {
	test := NewRouteTest(t)
	downloadsBefore := currentDownloads(test)

	testSite := startTestSiteWithDownloads(t, []*DownloadMeta{
		&DownloadMeta{
			Title: "Updated Download",
			URL:   "/my/special/new/url",
		},
	})
	defer testSite.Close()
	test.Config.SiteURL = testSite.URL

	url := fmt.Sprintf("/orders/%s/downloads/refresh", test.Data.firstOrder.ID)
	recorder := test.TestEndpoint(http.MethodPost, url, nil, test.Data.testUserToken)
	body, err := ioutil.ReadAll(recorder.Body)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, recorder.Code, "Failure: %s", string(body))

	downloadsAfter := currentDownloads(test)

	assert.Equal(t, len(downloadsBefore)+1, len(downloadsAfter))
	exists := false
	for _, download := range downloadsAfter {
		found := false
		for _, prev := range downloadsBefore {
			if download.ID == prev.ID {
				found = true
				break
			}
		}
		if !found {
			assert.Equal(t, "/my/special/new/url", download.URL)
			assert.Equal(t, "123-i-can-fly-456", download.Sku)
			exists = true
		}
	}
	assert.True(t, exists)
}
