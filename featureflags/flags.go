package featureflags

import (
	"testing"
	"time"

	"gopkg.in/launchdarkly/go-sdk-common.v2/lduser"
	ld "gopkg.in/launchdarkly/go-server-sdk.v5"
)

var globalClient *ld.LDClient

var (
	UseAltDB = Flag{key: "gocommerce_use_alt_db"}
)

type Flag struct {
	override     *bool
	overrideKeys []string

	key string
}

func (f *Flag) Enabled(userID string) bool {
	if f.override != nil {
		if len(f.overrideKeys) == 0 {
			// override is for all
			return *f.override
		}
		for _, id := range f.overrideKeys {
			if id == userID {
				return *f.override
			}
		}
		// no one else matched, go with default val
		return false
	}
	return Enabled(string(f.key), userID)
}

func (f *Flag) Override(t *testing.T, val bool, ids ...string) {
	f.override = &val
	t.Cleanup(func() {
		f.override = nil
	})
}

func Init(apiKey string) error {
	client, err := ld.MakeClient(apiKey, time.Second*30)
	if err != nil {
		return err
	}
	globalClient = client
	return nil
}

func Enabled(key, userID string) bool {
	if globalClient == nil {
		return false
	}

	val, _ := globalClient.BoolVariation(key, lduser.NewUser(userID), false)
	return val
}
