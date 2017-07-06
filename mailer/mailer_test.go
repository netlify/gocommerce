package mailer

import (
	"testing"

	"github.com/netlify/gocommerce/conf"
	"github.com/stretchr/testify/assert"
)

func TestNoopMailer(t *testing.T) {
	conf := &conf.Configuration{}
	m := NewMailer(conf)
	assert.IsType(t, &noopMailer{}, m)
}

func TestTemplateMailer(t *testing.T) {
	conf := &conf.Configuration{}
	conf.Mailer.AdminEmail = "test@example.com"
	conf.Mailer.Host = "localhost"
	conf.Mailer.Port = 25
	m := NewMailer(conf)
	assert.IsType(t, &mailer{}, m)
}
