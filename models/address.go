package models

import (
	"fmt"
	"strings"
	"time"
)

// AddressRequest is the raw address data
type AddressRequest struct {
	Name string `json:"name"`

	Company  string `json:"company"`
	Address1 string `json:"address1"`
	Address2 string `json:"address2"`
	City     string `json:"city"`
	Country  string `json:"country"`
	State    string `json:"state"`
	Zip      string `json:"zip"`
}

// Address is a stored address, reusable with an ID.
type Address struct {
	AddressRequest

	ID string `json:"id"`

	User   *User  `json:"-"`
	UserID string `json:"-"`

	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at"`
}

// TableName returns the table name used for the Address model
func (Address) TableName() string {
	return tableName("addresses")
}

// Validate validates the AddressRequest model
func (a AddressRequest) Validate() error {
	required := map[string]string{
		"name":    a.Name,
		"address": a.Address1,
		"country": a.Country,
		"city":    a.City,
		"zip":     a.Zip,
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
