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

var firstOrder = models.NewOrder("session1", testUser.Email, "usd")
var firstTransaction = models.NewTransaction(firstOrder)
var firstLineItem = models.LineItem{
	ID:          11,
	OrderID:     firstOrder.ID,
	Title:       "batwing",
	SKU:         "123-i-can-fly-456",
	Type:        "plane",
	Description: "it's the batwing.",
	Price:       12,
	Quantity:    2,
	Path:        "/i/believe/i/can/fly",
}

var secondOrder = models.NewOrder("session2", testUser.Email, "usd")
var secondTransaction = models.NewTransaction(secondOrder)
var secondLineItem1 = models.LineItem{
	ID:          21,
	OrderID:     secondOrder.ID,
	Title:       "tumbler",
	SKU:         "456-i-rollover-all-things",
	Type:        "tank",
	Description: "OMG yes",
	Price:       5,
	Quantity:    2,
	Path:        "/i/crush/villians/dreams",
}
var secondLineItem2 = models.LineItem{
	ID:          22,
	OrderID:     secondOrder.ID,
	Title:       "utility belt",
	SKU:         "234-fancy-belts",
	Type:        "clothes",
	Description: "stlyish but still useful",
	Price:       45,
	Quantity:    1,
	Path:        "/i/hold/the/universe/on/my/waist",
}

func loadTestData(db *gorm.DB) {
	db.Create(&testUser)
	db.Create(&testAddress)

	firstOrder.ID = "first-order"
	firstOrder.LineItems = []*models.LineItem{&firstLineItem}
	firstOrder.CalculateTotal(&models.SiteSettings{})
	firstOrder.BillingAddress = testAddress
	firstOrder.ShippingAddress = testAddress
	db.Create(&firstLineItem)
	db.Create(firstTransaction)
	db.Create(firstOrder)
	firstOrder.User = &testUser

	db.Create(&firstLineItem)
	db.Create(firstTransaction)
	db.Create(firstOrder)

	secondOrder.ID = "second-order"
	secondOrder.LineItems = []*models.LineItem{&secondLineItem1, &secondLineItem2}
	secondOrder.CalculateTotal(&models.SiteSettings{})
	secondOrder.BillingAddress = testAddress
	secondOrder.ShippingAddress = testAddress
	secondOrder.User = &testUser
	db.Create(&secondLineItem1)
	db.Create(&secondLineItem2)
	db.Create(secondTransaction)
	db.Create(secondOrder)
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
