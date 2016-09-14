package testutils

import "github.com/netlify/gocommerce/models"

func GetTestAddress() *models.Address {
	return &models.Address{
		LastName:  "parker",
		FirstName: "Peter",
		Address1:  "123 spidey lane",
		Country:   "marvel-land",
		City:      "new york",
		Zip:       "10007",
	}
}
