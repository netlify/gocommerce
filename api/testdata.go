package api

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
	"github.com/netlify/gocommerce/models"
)

var testUser = models.User{
	ID:    "i-am-batman",
	Email: "bruce@wayneindustries.com",
}

var testAddress = models.Address{
	ID:       "first-address",
	LastName: "wayne",
	Address1: "123 cave way",
	Country:  "dcland",
	City:     "gotham",
	Zip:      "324234",
	User:     &testUser,
}

// All necessary for *an* order
var firstOrder = models.NewOrder("session1", testUser.Email, "usd")
var firstLineItem = models.LineItem{
	ID:          798345,
	OrderID:     firstOrder.ID,
	Title:       "batwing",
	SKU:         "123-i-can-fly-456",
	Type:        "plane",
	Description: "it's the batwing.",
	Price:       123123123,
	Quantity:    2,
	Path:        "/i/believe/i/can/fly",
}
var firstTransaction = models.NewTransaction(firstOrder)

func loadTestData(db *gorm.DB) {
	db.Create(&testUser)
	db.Create(&testAddress)

	firstOrder.LineItems = []*models.LineItem{&firstLineItem}
	firstOrder.CalculateTotal(&models.SiteSettings{})
	firstOrder.BillingAddress = testAddress
	firstOrder.ShippingAddress = testAddress
	db.Create(&firstLineItem)
	db.Create(firstTransaction)
	db.Create(firstOrder)
}

func token(id, email string, groups *[]string) *jwt.Token {
	claims := &JWTClaims{
		ID:     id,
		Email:  email,
		Groups: []string{},
	}
	if groups != nil {
		for _, g := range *groups {
			claims.Groups = append(claims.Groups, g)
		}
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t
}
