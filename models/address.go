package models

import (
	"fmt"
	"strings"
	"time"
)

type AddressRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`

	Company  string `json:"company"`
	Address1 string `json:"address1"`
	Address2 string `json:"address2"`
	City     string `json:"city"`
	Country  string `json:"country"`
	State    string `json:"state"`
	Zip      string `json:"zip"`
}

type Address struct {
	AddressRequest

	ID string `json:"id"`

	User   *User  `json:"-"`
	UserID string `json:"-"`

	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at"`
}

func (Address) TableName() string {
	return tableName("addresses")
}

func (a AddressRequest) Validate() error {
	required := map[string]string{
		"last name": a.LastName,
		"address":   a.Address1,
		"country":   a.Country,
		"city":      a.City,
		"zip":       a.Zip,
	}

	missing := []string{}
	for name, val := range required {
		if val == "" {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("Required field missing: " + strings.Join(missing, ","))
	}

	return nil
}
