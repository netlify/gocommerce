package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"

	"github.com/netlify/netlify-commerce/conf"
)

func TestTraceWrapper(t *testing.T) {
	l, hook := test.NewNullLogger()
	testConfig := new(conf.Configuration)

	api := NewAPI(testConfig, nil, nil)
	api.log = logrus.NewEntry(l)

	server := httptest.NewServer(api.handler)
	defer server.Close()

	client := http.Client{}
	rsp, err := client.Get(server.URL)
	if assert.NoError(t, err) {
		assert.Equal(t, 200, rsp.StatusCode)

		assert.True(t, len(hook.Entries) > 0)

		for _, entry := range hook.Entries {
			if _, ok := entry.Data["request_id"]; !ok {
				assert.Fail(t, "expected entry: request_id")
			}
			expected := map[string]string{
				"method": "GET",
				"path":   "/",
			}
			for k, v := range expected {
				if value, ok := entry.Data[k]; ok {
					assert.Equal(t, v, value)
				} else {
					assert.Fail(t, "expected entry: "+k)
				}
			}
		}
	}
}
