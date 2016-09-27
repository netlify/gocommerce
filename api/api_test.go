package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/test"
	"github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"

	"github.com/netlify/netlify-commerce/conf"
)

func TestTraceWrapper(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)

	l, hook := test.NewNullLogger()
	testConfig := new(conf.Configuration)
	testContext := withToken(context.Background(), testToken("batman", "bruce@wayne.com", nil))

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "http://something/somewhere", nil)
	assert.Nil(t, err)

	api := NewAPI(testConfig, nil, nil)
	api.log = logrus.NewEntry(l)

	api.trace(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, testConfig, getConfig(ctx))
		assert.NotEqual(t, "", getRequestID(ctx))

		log := getLogger(ctx)
		log.Info("something happened")
	})(testContext, recorder, req)

	for _, entry := range hook.Entries {
		// on every entry
		if _, ok := entry.Data["request_id"]; !ok {
			assert.Fail(t, "expected entry: request_id")
		}
		expected := map[string]string{
			"claim_id":    "batman",
			"claim_email": "bruce@wayne.com",
			"method":      "GET",
			"path":        "/somewhere",
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

func testToken(id, email string, groups *[]string) *jwt.Token {
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
