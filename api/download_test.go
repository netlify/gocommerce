package api

import (
	"net/http"
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
