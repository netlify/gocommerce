package cmd

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/netlify/gocommerce/api"
	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/mailer"
	"github.com/netlify/gocommerce/models"
	"github.com/spf13/cobra"
	stripe "github.com/stripe/stripe-go"
)

var serveCmd = cobra.Command{
	Use:  "serve",
	Long: "Start API server",
	Run: func(cmd *cobra.Command, args []string) {
		execWithConfig(cmd, serve)
	},
}

func serve(config *conf.Configuration) {
	db, err := models.Connect(config)
	if err != nil {
		logrus.Fatalf("Error opening database: %+v", err)
	}

	mailer := mailer.NewMailer(config)

	api := api.NewAPIWithVersion(config, db.Debug(), mailer, Version)

	stripe.Key = config.Payment.Stripe.SecretKey

	l := fmt.Sprintf("%v:%v", config.API.Host, config.API.Port)
	logrus.Infof("Netlify Commerce API started on: %s", l)
	api.ListenAndServe(l)
}
