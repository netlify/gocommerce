package conf

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigWithOverrides(t *testing.T) {
	original := Configuration{}
	original.SiteURL = "http://example.com"
	original.JWT.Secret = "jwt-secret"
	original.DB.Driver = "db-driver"
	original.DB.ConnURL = "conn-url"
	original.API.Host = "api-host"
	original.API.Port = 12356
	original.Mailer.Host = "mailer-host"
	original.Mailer.Port = 789789
	original.Mailer.User = "mailer-user"
	original.Mailer.Pass = "mailer-pass"
	original.Mailer.AdminEmail = "admin-email"
	original.Payment.Stripe.SecretKey = "stripe-secret"

	tmpfile, err := ioutil.TempFile("", "gocommerce-test")
	assert.Nil(t, err)

	fname := tmpfile.Name() + ".json"
	err = os.Rename(tmpfile.Name(), fname)
	assert.Nil(t, err)
	defer os.Remove(fname)

	content, err := json.Marshal(&original)
	assert.Nil(t, err)

	err = ioutil.WriteFile(fname, content, 0755)
	assert.Nil(t, err)

	// override some values
	os.Setenv("NETLIFY_COMMERCE_SITE_URL", "http://env.com")
	os.Setenv("NETLIFY_COMMERCE_JWT_SECRET", "env-jwt-secret")
	os.Setenv("NETLIFY_COMMERCE_DB_DRIVER", "env-db-driver")
	os.Setenv("NETLIFY_COMMERCE_API_PORT", "456456")
	os.Setenv("NETLIFY_COMMERCE_MAILER_USER", "env-mailer-user")
	os.Setenv("NETLIFY_COMMERCE_PAYMENT_STRIPE_SECRET_KEY", "env-stripe-secret")

	config, err := Load(fname)
	assert.Nil(t, err)
	assert.NotNil(t, config)

	// check we loaded from the file
	assert.Equal(t, config.DB.ConnURL, original.DB.ConnURL)
	assert.Equal(t, config.API.Host, original.API.Host)
	assert.Equal(t, config.Mailer.Host, original.Mailer.Host)
	assert.Equal(t, config.Mailer.Port, original.Mailer.Port)
	assert.Equal(t, config.Mailer.Pass, original.Mailer.Pass)
	assert.Equal(t, config.Mailer.AdminEmail, original.Mailer.AdminEmail)

	// check we got the overrides
	assert.Equal(t, "http://env.com", config.SiteURL)
	assert.Equal(t, "env-jwt-secret", config.JWT.Secret)
	assert.Equal(t, "env-db-driver", config.DB.Driver)
	assert.EqualValues(t, 456456, config.API.Port)
	assert.Equal(t, "env-mailer-user", config.Mailer.User)
	assert.Equal(t, "env-stripe-secret", config.Payment.Stripe.SecretKey)
}
