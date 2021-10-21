package featureflags

import (
	"time"

	"gopkg.in/launchdarkly/go-sdk-common.v2/lduser"
	ld "gopkg.in/launchdarkly/go-server-sdk.v5"
)

var globalClient *ld.LDClient

const (
	UseAltDB Flag = "gocommerce_use_alt_db"
)

type Flag string

func (f Flag) Enabled(userID string) bool {
	return Enabled(string(f), userID)
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
