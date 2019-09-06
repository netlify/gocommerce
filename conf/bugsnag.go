package conf

import (
	"github.com/bugsnag/bugsnag-go"
	logrus_bugsnag "github.com/shopify/logrus-bugsnag"
	"github.com/sirupsen/logrus"
)

type BugSnagConfig struct {
	Environment string
	APIKey      string `envconfig:"api_key"`
}

func AddBugSnagHook(config *BugSnagConfig) error {
	if config == nil || config.APIKey == "" {
		return nil
	}

	bugsnag.Configure(bugsnag.Configuration{
		APIKey:       config.APIKey,
		ReleaseStage: config.Environment,
		PanicHandler: func() {}, // this is to disable panic handling. The lib was forking and restarting the process (causing races)
	})
	hook, err := logrus_bugsnag.NewBugsnagHook()
	if err != nil {
		return err
	}
	logrus.AddHook(hook)
	logrus.Debug("Added bugsnag hook")
	return nil
}
