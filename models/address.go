package models

import "time"

type Address struct {
	ID string `json:"id"`

	User   *User  `json:"-"`
	UserID string `json:"-"`

	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`

	Company string `json:"company"`
	Address string `json:"address"`
	City    string `json:"city"`
	Country string `json:"country"`
	State   string `json:"state"`
	Zip     string `json:"zip"`

	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at"`
}

func (a *Address) Valid() bool {
	if a.FirstName == "" || a.LastName == "" {
		return false
	}
	if a.Address == "" {
		return false
	}
	if a.Country == "" || a.City == "" || a.Zip == "" {
		return false
	}
	return true
}
