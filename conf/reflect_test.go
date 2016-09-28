package conf

import (
	"reflect"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestSimpleValues(t *testing.T) {
	c := struct {
		Simple string `json:"simple"`
	}{}

	viper.SetDefault("simple", "i am a simple string")

	assert.Nil(t, recursivelySet(reflect.ValueOf(&c), ""))
	assert.Equal(t, "i am a simple string", c.Simple)
}

func TestNestedValues(t *testing.T) {
	c := struct {
		Simple string `json:"simple"`
		Nested struct {
			BoolVal   bool   `json:"bool"`
			StringVal string `json:"string"`
			NumberVal int    `json:"number"`
		} `json:"nested"`
	}{}

	viper.SetDefault("simple", "simple")
	viper.SetDefault("nested.bool", true)
	viper.SetDefault("nested.string", "i am a simple string")
	viper.SetDefault("nested.number", 4)

	assert.Nil(t, recursivelySet(reflect.ValueOf(&c), ""))
	assert.Equal(t, "simple", c.Simple)
	assert.Equal(t, 4, c.Nested.NumberVal)
	assert.Equal(t, "i am a simple string", c.Nested.StringVal)
	assert.Equal(t, true, c.Nested.BoolVal)
}
