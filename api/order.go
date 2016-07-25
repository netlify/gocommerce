package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/dgrijalva/jwt-go"
	"github.com/guregu/kami"
	"github.com/jinzhu/gorm"
	"github.com/netlify/gocommerce/models"
	"github.com/pborman/uuid"

	"golang.org/x/net/context"
)

type OrderParams struct {
	SessionID string `json:"session_id"`

	Email string `json:"email"`

	ShippingAddressID string          `json:"shipping_address_id"`
	ShippingAddress   *models.Address `json:"shipping_address"`

	BillingAddressID string          `json:"billing_address_id"`
	BillingAddress   *models.Address `json:"billing_address"`

	LineItems []models.LineItem `json:"line_items"`
}

type verificationError struct {
	err   error
	mutex sync.Mutex
}

func (e *verificationError) setError(err error) {
	e.mutex.Lock()
	e.err = err
	e.mutex.Unlock()
}

func (a *API) OrderList(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	token := getToken(ctx)
	if token == nil {
		UnauthorizedError(w, "Order History Requires Authentication")
		return
	}

	claims := token.Claims.(*JWTClaims)

	var orders []models.Order
	result := a.db.Preload("LineItems").Preload("ShippingAddress").Where("user_id = ?", claims.ID).Find(&orders)
	if result.Error != nil {
		InternalServerError(w, fmt.Sprintf("Error during database query: %v", result.Error))
		return
	}

	sendJSON(w, 200, orders)
}

func (a *API) OrderView(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	token := getToken(ctx)
	id := kami.Param(ctx, "id")

	order := &models.Order{}
	if result := a.db.Preload("LineItems").First(order, "id = ?", id); result.Error != nil {
		if result.RecordNotFound() {
			NotFoundError(w, "Order not found")
		} else {
			InternalServerError(w, fmt.Sprintf("Error during database query: %v", result.Error))
		}
		return
	}

	if order.UserID != "" && (order.UserID != userIDFromToken(token)) {
		UnauthorizedError(w, "You don't have access to this order")
		return
	}

	sendJSON(w, 200, order)
}

// OrderCreate endpoint
func (a *API) OrderCreate(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	params := &OrderParams{}
	jsonDecoder := json.NewDecoder(r.Body)
	err := jsonDecoder.Decode(params)
	if err != nil {
		BadRequestError(w, fmt.Sprintf("Could not read Order params: %v", err))
		return
	}

	token := getToken(ctx)

	order := models.NewOrder(params.SessionID, params.Email)

	tx := a.db.Begin()

	user := &models.User{}
	if httpError := a.setUserIDFromToken(tx, user, order, token); httpError != nil {
		sendJSON(w, httpError.Code, err)
		return
	}

	if httpError := a.createLineItems(tx, order, params.LineItems); httpError != nil {
		sendJSON(w, httpError.Code, httpError)
		return
	}

	shippingID, httpError := a.processAddress(tx, order, params.ShippingAddress, params.ShippingAddressID)
	if httpError != nil {
		sendJSON(w, httpError.Code, httpError)
		return
	}
	if shippingID == "" {
		BadRequestError(w, "Shipping Address Required")
		return
	}
	order.ShippingAddressID = shippingID

	billingID, httpError := a.processAddress(tx, order, params.BillingAddress, params.BillingAddressID)
	if httpError != nil {
		sendJSON(w, httpError.Code, httpError)
		return
	}
	if billingID != "" {
		order.BillingAddressID = billingID
	} else {
		order.BillingAddressID = shippingID
	}

	tx.Create(order)
	tx.Commit()

	sendJSON(w, 200, order)
}

func (a *API) setUserIDFromToken(tx *gorm.DB, user *models.User, order *models.Order, token *jwt.Token) *HTTPError {
	claims := token.Claims.(*JWTClaims)

	if token != nil {
		if claims.ID == "" {
			tx.Rollback()
			return &HTTPError{Code: 400, Message: fmt.Sprintf("Token had an invalid ID: %v", claims.ID)}
		}
		order.UserID = claims.ID
		if result := tx.First(user, "id = ?", claims.ID); result.Error != nil {
			if result.RecordNotFound() {
				user.ID = claims.ID
				if claims.Email != "" {
					user.Email = claims.Email
				} else {
					order.Email = user.Email
				}
				tx.Create(user)
			} else {
				tx.Rollback()
				return &HTTPError{Code: 500, Message: fmt.Sprintf("Token had an invalid ID: %v", result.Error)}
			}
		}
	}
	return nil
}

func (a *API) createLineItems(tx *gorm.DB, order *models.Order, items []models.LineItem) *HTTPError {
	for _, item := range items {
		item.ID = 0 // Making sure you can't set this via params
		item.OrderID = order.ID
		order.SubTotal = order.SubTotal + item.Price*item.Quantity
		if err := tx.Create(&item).Error; err != nil {
			tx.Rollback()
			return &HTTPError{Code: 500, Message: fmt.Sprintf("Error creating line item: %v", err)}
		}
	}
	order.Total = order.SubTotal
	return nil
}

func (a *API) processAddress(tx *gorm.DB, order *models.Order, address *models.Address, id string) (string, *HTTPError) {
	if id != "" {
		if result := tx.First(address, "id = ?", id); result.Error != nil {
			tx.Rollback()
			return "", &HTTPError{Code: 400, Message: fmt.Sprintf("Bad address id: %v", id)}
		}
		if address.UserID == "" {
			address.UserID = order.UserID
			tx.Save(address)
		} else if order.UserID != address.UserID {
			tx.Rollback()
			return "", &HTTPError{Code: 400, Message: fmt.Sprintf("Bad address id: %v", id)}
		}
	} else {
		if address == nil {
			return "", nil
		} else if address.Valid() {
			address.ID = uuid.NewRandom().String()
			tx.Create(address)
		} else {
			tx.Rollback()
			return "", &HTTPError{Code: 400, Message: "Failed to validate address"}
		}
	}
	return address.ID, nil
}
