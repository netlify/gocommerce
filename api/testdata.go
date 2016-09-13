package api

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
	"github.com/netlify/gocommerce/models"
)

var testUser models.User
var testAddress models.Address

var firstOrder *models.Order
var firstTransaction *models.Transaction
var firstLineItem models.LineItem

var secondOrder *models.Order
var secondTransaction *models.Transaction
var secondLineItem1 models.LineItem
var secondLineItem2 models.LineItem

func loadTestData(db *gorm.DB) {
	testUser = models.User{
		ID:    "i-am-batman",
		Email: "bruce@wayneindustries.com",
	}

	testAddress = models.Address{
		ID:       "first-address",
		LastName: "wayne",
		Address1: "123 cave way",
		Country:  "dcland",
		City:     "gotham",
		Zip:      "324234",
		User:     &testUser,
	}

	firstOrder = models.NewOrder("session1", testUser.Email, "usd")
	firstTransaction = models.NewTransaction(firstOrder)
	firstLineItem = models.LineItem{
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

	secondOrder = models.NewOrder("session2", testUser.Email, "usd")
	secondTransaction = models.NewTransaction(secondOrder)
	secondLineItem1 = models.LineItem{
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
	secondLineItem2 = models.LineItem{
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

	db.Create(&testUser)
	db.Create(&testAddress)

	firstOrder.ID = "first-order"
	firstOrder.LineItems = []*models.LineItem{&firstLineItem}
	firstOrder.CalculateTotal(&models.SiteSettings{})
	firstOrder.BillingAddress = testAddress
	firstOrder.ShippingAddress = testAddress
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
