package conf

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGlobalConfigWithOverrides(t *testing.T) {
	original := GlobalConfiguration{}
	original.DB.Driver = "db-driver"
	original.DB.ConnURL = "conn-url"
	original.API.Host = "api-host"
	original.API.Port = 12356

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
	os.Setenv("GOCOMMERCE_DB_DRIVER", "env-db-driver")
	os.Setenv("GOCOMMERCE_API_PORT", "456456")

	config, err := LoadGlobal(fname)
	assert.Nil(t, err)
	assert.NotNil(t, config)

	// check we loaded from the file
	assert.Equal(t, config.DB.ConnURL, original.DB.ConnURL)
	assert.Equal(t, config.API.Host, original.API.Host)

	// check we got the overrides
	assert.Equal(t, "env-db-driver", config.DB.Driver)
	assert.EqualValues(t, 456456, config.API.Port)
}

func TestConfigWithOverrides(t *testing.T) {
	instance := Configuration{}
	instance.SiteURL = "http://example.com"
	instance.JWT.Secret = "jwt-secret"
	instance.Mailer.Host = "mailer-host"
	instance.Mailer.Port = 789789
	instance.Mailer.User = "mailer-user"
	instance.Mailer.Pass = "mailer-pass"
	instance.Mailer.AdminEmail = "admin-email"
	instance.Payment.Stripe.SecretKey = "stripe-secret"

	tmpfile, err := ioutil.TempFile("", "gocommerce-test")
	assert.Nil(t, err)

	fname := tmpfile.Name() + ".json"
	err = os.Rename(tmpfile.Name(), fname)
	assert.Nil(t, err)
	defer os.Remove(fname)

	content, err := json.Marshal(&instance)
	assert.Nil(t, err)

	err = ioutil.WriteFile(fname, content, 0755)
	assert.Nil(t, err)

	// override some values
	os.Setenv("GOCOMMERCE_SITE_URL", "http://env.com")
	os.Setenv("GOCOMMERCE_JWT_SECRET", "env-jwt-secret")
	os.Setenv("GOCOMMERCE_MAILER_USER", "env-mailer-user")
	os.Setenv("GOCOMMERCE_PAYMENT_STRIPE_SECRET_KEY", "env-stripe-secret")

	config, err := Load(fname)
	assert.Nil(t, err)
	assert.NotNil(t, config)

	// check we loaded from the file
	assert.Equal(t, config.Mailer.Host, instance.Mailer.Host)
	assert.Equal(t, config.Mailer.Port, instance.Mailer.Port)
	assert.Equal(t, config.Mailer.Pass, instance.Mailer.Pass)
	assert.Equal(t, config.Mailer.AdminEmail, instance.Mailer.AdminEmail)

	// check we got the overrides
	assert.Equal(t, "http://env.com", config.SiteURL)
	assert.Equal(t, "env-jwt-secret", config.JWT.Secret)
	assert.Equal(t, "env-mailer-user", config.Mailer.User)
	assert.Equal(t, "env-stripe-secret", config.Payment.Stripe.SecretKey)
}
