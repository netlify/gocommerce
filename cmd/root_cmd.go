package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/stripe/stripe-go"

	"github.com/netlify/gocommerce/api"
	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/mailer"
	"github.com/netlify/gocommerce/models"
)

// RootCmd will run the log streamer
var RootCmd = cobra.Command{
	Use:  "gocommerce",
	Long: "A service that will validate restful transactions and send them to stripe.",
	Run: func(cmd *cobra.Command, args []string) {
		configFile, err := cmd.PersistentFlags().GetString("config")
		if err != nil {
			log.Fatal("Failed to find config flag %v", err)
		}

		config, err := conf.Load(configFile)
		if err != nil {
			log.Fatal("Failed to load configration: %v", err)
		}
		execute(config)
	},
}

// InitCommandFlags will add all the flags to the different commands
func InitCommandFlags() {
	RootCmd.PersistentFlags().StringP("config", "c", "", "The configuration file")
}

func execute(config *conf.Configuration) {
	db, err := models.Connect(config)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}

	mailer := mailer.NewMailer(config)

	api := api.NewAPI(config, db.Debug(), mailer)

	stripe.Key = config.Payment.Stripe.SecretKey

	api.ListenAndServe(fmt.Sprintf("%v:%v", config.API.Host, config.API.Port))
}
