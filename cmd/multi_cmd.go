package cmd

import (
	"context"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/netlify/gocommerce/api"
	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/models"
	"github.com/spf13/cobra"
)

var multiCmd = cobra.Command{
	Use:  "multi",
	Long: "Start multi-tenant API server",
	Run: func(cmd *cobra.Command, args []string) {
		execWithConfig(cmd, multi)
	},
}

func multi(config *conf.GlobalConfiguration) {
	db, err := models.Connect(config)
	if err != nil {
		logrus.Fatalf("Error opening database: %+v", err)
	}

	bgDB, err := models.Connect(config)
	if err != nil {
		logrus.Fatalf("Error opening database: %+v", err)
	}

	api := api.NewAPIWithVersion(context.Background(), config, db.Debug(), Version)

	l := fmt.Sprintf("%v:%v", config.API.Host, config.API.Port)
	logrus.Infof("GoCommerce API started on: %s", l)

	models.RunHooks(bgDB, logrus.WithField("component", "hooks"))

	logrus.WithError(api.ListenAndServe(l)).Fatal("Error listening")
}
