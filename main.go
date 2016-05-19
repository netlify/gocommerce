package main

import (
	"fmt"
	"log"

	"github.com/netlify/gocommerce/api"
	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/mailer"
	"github.com/netlify/gocommerce/models"
	"github.com/stripe/stripe-go"
)

func main() {
	config, err := conf.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	db, err := models.Connect(config)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}

	mailer := mailer.NewMailer(config)

	api := api.NewAPI(config, db.Debug(), mailer)

	stripe.Key = config.Payment.Stripe.SecretKey

	api.ListenAndServe(fmt.Sprintf("%v:%v", config.API.Host, config.API.Port))
}
