package claims

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClaims(t *testing.T) {
	b, err := ioutil.ReadFile("test/jwt_payload_fixture.json")
	assert.NoError(t, err)

	var claims map[string]interface{}
	err = json.Unmarshal(b, &claims)

	required := map[string]string{
		"app_metadata.subscription.plan": "smashing",
	}

	matches := HasClaims(claims, required)
	assert.True(t, matches)

	required = map[string]string{
		"app_metadata.subscription.plan": "member",
	}

	matches = HasClaims(claims, required)
	assert.False(t, matches)
}
