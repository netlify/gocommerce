package testutils

import (
	"github.com/jinzhu/gorm"
	"github.com/netlify/gocommerce/models"
)

var TestUser models.User
var TestAddress models.Address

var FirstOrder *models.Order
var FirstTransaction *models.Transaction
var FirstLineItem models.LineItem

var SecondOrder *models.Order
var SecondTransaction *models.Transaction
var SecondLineItem1 models.LineItem
var SecondLineItem2 models.LineItem

func LoadTestData(db *gorm.DB) {
	TestUser = models.User{
		ID:    "i-am-batman",
		Email: "bruce@wayneindustries.com",
	}

	TestAddress = models.Address{
		AddressRequest: models.AddressRequest{
			LastName: "wayne",
			Address1: "123 cave way",
			Country:  "dcland",
			City:     "gotham",
			Zip:      "324234",
		},
		ID:   "first-address",
		User: &TestUser,
	}

	FirstOrder = models.NewOrder("session1", TestUser.Email, "usd")
	FirstTransaction = models.NewTransaction(FirstOrder)
	FirstLineItem = models.LineItem{
		ID:          11,
		OrderID:     FirstOrder.ID,
		Title:       "batwing",
		SKU:         "123-i-can-fly-456",
		Type:        "plane",
		Description: "it's the batwing.",
		Price:       12,
		Quantity:    2,
		Path:        "/i/believe/i/can/fly",
	}

	SecondOrder = models.NewOrder("session2", TestUser.Email, "usd")
	SecondTransaction = models.NewTransaction(SecondOrder)
	SecondLineItem1 = models.LineItem{
		ID:          21,
		OrderID:     SecondOrder.ID,
		Title:       "tumbler",
		SKU:         "456-i-rollover-all-things",
		Type:        "tank",
		Description: "OMG yes",
		Price:       5,
		Quantity:    2,
		Path:        "/i/crush/villians/dreams",
	}
	SecondLineItem2 = models.LineItem{
		ID:          22,
		OrderID:     SecondOrder.ID,
		Title:       "utility belt",
		SKU:         "234-fancy-belts",
		Type:        "clothes",
		Description: "stlyish but still useful",
		Price:       45,
		Quantity:    1,
		Path:        "/i/hold/the/universe/on/my/waist",
	}

	db.Create(&TestUser)
	db.Create(&TestAddress)

	FirstOrder.ID = "first-order"
	FirstOrder.LineItems = []*models.LineItem{&FirstLineItem}
	FirstOrder.CalculateTotal(&models.SiteSettings{})
	FirstOrder.BillingAddress = TestAddress
	FirstOrder.ShippingAddress = TestAddress
	FirstOrder.User = &TestUser
	db.Create(&FirstLineItem)
	db.Create(FirstTransaction)
	db.Create(FirstOrder)

	SecondOrder.ID = "second-order"
	SecondOrder.LineItems = []*models.LineItem{&SecondLineItem1, &SecondLineItem2}
	SecondOrder.CalculateTotal(&models.SiteSettings{})
	SecondOrder.BillingAddress = TestAddress
	SecondOrder.ShippingAddress = TestAddress
	SecondOrder.User = &TestUser
	db.Create(&SecondLineItem1)
	db.Create(&SecondLineItem2)
	db.Create(SecondTransaction)
	db.Create(SecondOrder)
}
