package cmd

import (
	"fmt"
	"log"

	"github.com/netlify/netlify-commerce/api"
	"github.com/netlify/netlify-commerce/conf"
	"github.com/netlify/netlify-commerce/mailer"
	"github.com/netlify/netlify-commerce/models"
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
		log.Fatalf("Error opening database: %v", err)
	}

	mailer := mailer.NewMailer(config)

	api := api.NewAPI(config, db.Debug(), mailer)

	stripe.Key = config.Payment.Stripe.SecretKey

	api.ListenAndServe(fmt.Sprintf("%v:%v", config.API.Host, config.API.Port))
}
