package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/Sirupsen/logrus"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/guregu/kami"
	"github.com/jinzhu/gorm"
	"github.com/mattes/vat"
	"github.com/netlify/netlify-commerce/models"
	"github.com/pborman/uuid"
)

type OrderLineItem struct {
	SKU      string `json:sku`
	Path     string `json:"path"`
	Quantity uint64 `json:"quantity"`
}

type OrderParams struct {
	SessionID string `json:"session_id"`

	Email string `json:"email"`

	ShippingAddressID string          `json:"shipping_address_id"`
	ShippingAddress   *models.Address `json:"shipping_address"`

	BillingAddressID string          `json:"billing_address_id"`
	BillingAddress   *models.Address `json:"billing_address"`

	VATNumber string `json:"vatnumber"`

	Data map[string]interface{} `json:"data"`

	LineItems []*OrderLineItem `json:"line_items"`

	Currency string `json:"currency"`
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

// OrderList can query based on
//  - orders since        &from=iso8601      - default = 0
//  - orders before       &to=iso8601        - default = now
//  - sort asc or desc    &sort=[asc | desc] - default = desc
func (a *API) OrderList(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	log := getLogger(ctx)

	var err error
	claims := getClaims(ctx)
	if claims == nil {
		unauthorizedError(w, "Order History Requires Authentication")
		log.Info("request made with no claims")
		return
	}

	params := r.URL.Query()
	query := orderQuery(a.db)
	query, err = parseOrderParams(query, params)
	if err != nil {
		log.WithError(err).Info("Bad query parameters in request")
		badRequestError(w, "Bad parameters in query: "+err.Error())
		return
	}

	// handle the admin info
	id := claims.ID
	if values, exists := params["user_id"]; exists {
		if isAdmin(ctx) {
			id = values[0]
			log.WithField("admin_id", claims.ID).Debugf("Making admin request for user %s by %s", id, claims.ID)
		} else {
			log.Warnf("Request for user id %s as user %s - but not an admin", values[0], id)
			badRequestError(w, "Can't request user id if you're not that user, or an admin")
			return
		}
	}
	query = query.Where("user_id = ?", id)
	log.WithField("query_user_id", id).Debug("URL parsed and query perpared")

	var orders []models.Order
	result := query.Find(&orders)
	if result.Error != nil {
		log.WithError(err).Warn("Error while querying database")
		internalServerError(w, fmt.Sprintf("Error during database query: %v", result.Error))
		return
	}

	log.WithField("order_count", len(orders)).Debugf("Successfully retrieved %d orders", len(orders))
	sendJSON(w, 200, orders)
}

// OrderView will request a specific order using the 'id' parameter.
// Only the owner of the order, an admin, or an anon order are allowed to be seen
func (a *API) OrderView(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	id := kami.Param(ctx, "id")
	log := getLogger(ctx).WithField("order_id", id)
	claims := getClaims(ctx)
	if claims == nil {
		log.Info("Request with no claims made")
		unauthorizedError(w, "Order History Requires Authentication")
		return
	}

	order := &models.Order{}
	if result := orderQuery(a.db).First(order, "id = ?", id); result.Error != nil {
		if result.RecordNotFound() {
			log.Debug("Requested record that doesn't exist")
			notFoundError(w, "Order not found")
		} else {
			log.WithError(result.Error).Warnf("Error while querying database: %s", result.Error.Error())
			internalServerError(w, fmt.Sprintf("Error during database query: %v", result.Error))
		}
		return
	}

	if order.UserID == "" || (order.UserID == claims.ID) || isAdmin(ctx) {
		log.Debugf("Successfully got order %s", order.ID)
		sendJSON(w, 200, order)
	} else {
		log.WithFields(logrus.Fields{
			"user_id":       claims.ID,
			"order_user_id": order.UserID,
		}).Warnf("Unauthorized access attempted for order %s by %s", order.ID, claims.ID)
		unauthorizedError(w, "You don't have access to this order")
	}
}

// OrderCreate endpoint
func (a *API) OrderCreate(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	log := getLogger(ctx)

	params := &OrderParams{Currency: "USD"}
	jsonDecoder := json.NewDecoder(r.Body)
	err := jsonDecoder.Decode(params)
	if err != nil {
		log.WithError(err).Infof("Failed to deserialize order params: %s", err.Error())
		badRequestError(w, "Could not read Order params: %v", err)
		return
	}

	claims := getClaims(ctx)

	order := models.NewOrder(params.SessionID, params.Email, params.Currency)
	log = log.WithFields(logrus.Fields{
		"order_id":   order.ID,
		"session_id": params.SessionID,
	})
	ctx = withLogger(ctx, log)

	log.WithFields(logrus.Fields{
		"email":    params.Email,
		"currency": params.Currency,
	}).Debug("Created order, starting to process request")
	tx := a.db.Begin()

	order.Email = params.Email
	httpError := setOrderEmail(tx, order, claims, log)
	if httpError != nil {
		tx.Rollback()
		log.WithError(err).Info("Failed to set the order email from the token")
		sendJSON(w, httpError.Code, err)
		return
	}

	log.WithField("order_user_id", order.UserID).Debug("Successfully set the order's ID")

	if httpError := a.createLineItems(ctx, tx, order, params.LineItems); httpError != nil {
		tx.Rollback()
		sendJSON(w, httpError.Code, httpError)
		return
	}
	log.WithField("subtotal", order.SubTotal).Debug("Successfully processed all the line items")

	shipping, httpError := a.processAddress(tx, order, params.ShippingAddress, params.ShippingAddressID)
	if httpError != nil {
		tx.Rollback()
		sendJSON(w, httpError.Code, httpError)
		return
	}
	if shipping == nil {
		badRequestError(w, "Shipping Address Required")
		return
	}
	order.ShippingAddressID = shipping.ID

	billing, httpError := a.processAddress(tx, order, params.BillingAddress, params.BillingAddressID)
	if httpError != nil {
		sendJSON(w, httpError.Code, httpError)
		return
	}
	if billing != nil {
		order.BillingAddressID = billing.ID
	} else {
		order.BillingAddressID = shipping.ID
	}

	if params.VATNumber != "" {
		valid, err := vat.IsValidVAT(params.VATNumber)
		if err != nil {
			tx.Rollback()
			internalServerError(w, fmt.Sprintf("Error verifying VAT number %v", err))
			return
		}
		if !valid {
			tx.Rollback()
			badRequestError(w, fmt.Sprintf("Vat number %v is not valid", order.VATNumber))
			return
		}
		order.VATNumber = params.VATNumber
	}

	if params.Data != nil {
		if err := order.UpdateOrderData(tx, &params.Data); err != nil {
			tx.Rollback()
			badRequestError(w, fmt.Sprintf("Bad order metadata %v", err))
			return
		}
	}

	tx.Create(order)
	tx.Commit()

	log.Infof("Successfully created order %s", order.ID)
	sendJSON(w, 201, order)
}

func (a *API) setUserIDFromToken(tx *gorm.DB, user *models.User, order *models.Order, token *jwt.Token) *HTTPError {
	if token != nil {
		claims := token.Claims.(*JWTClaims)
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

// An order's email is determined by a few things. The rules guiding it are:
// 1 - if no claims are provided then the one in the params is used (for anon orders)
// 2 - if claims are provided they must be a valid user id
// 3 - if that user doesn't exist then a user will be created with the id/email specified.
//     if the user doesn't have an email, the one from the order is used
// 4 - if the order doesn't have an email, but the user does, we will use that one
//
func setOrderEmail(tx *gorm.DB, order *models.Order, claims *JWTClaims, log *logrus.Entry) *HTTPError {
	if claims == nil {
		log.Debug("No claims provided, proceeding as an anon request")
	} else {
		if claims.ID == "" {
			return httpError(400, "Token had an invalid ID: %s", claims.ID)
		}

		log = log.WithField("user_id", claims.ID)
		order.UserID = claims.ID

		user := new(models.User)
		result := tx.First(user, "id = ?", claims.ID)
		if result.RecordNotFound() {
			log.Debugf("Didn't find a user for id %s ~ going to create one", claims.ID)
			user.ID = claims.ID
			user.Email = claims.Email
			tx.Create(user)
		} else if result.Error != nil {
			log.WithError(result.Error).Warnf("Unexpected error from the db while querying for user id %d", user.ID)
			return httpError(500, "Token had an invalid ID: %v", result.Error)
		}

		if order.Email == "" {
			order.Email = user.Email
		}
	}

	if order.Email == "" {
		return httpError(400, "Either the order parameters or the user must provide an email")
	}
	return nil
}

func (a *API) createLineItems(ctx context.Context, tx *gorm.DB, order *models.Order, items []*OrderLineItem) *HTTPError {
	sem := make(chan int, MaxConcurrentLookups)
	var wg sync.WaitGroup
	sharedErr := verificationError{}
	for _, item := range items {
		lineItem := &models.LineItem{SKU: item.SKU, Quantity: item.Quantity, Path: item.Path, OrderID: order.ID}
		order.LineItems = append(order.LineItems, lineItem)
		sem <- 1
		wg.Add(1)
		go func(item *models.LineItem) {
			defer func() {
				wg.Done()
				<-sem
			}()
			// Stop doing any work if there's already an error
			if sharedErr.err != nil {
				return
			}

			if err := a.processLineItem(ctx, order, item); err != nil {
				sharedErr.setError(err)
			}
		}(lineItem)
	}
	wg.Wait()

	if sharedErr.err != nil {
		tx.Rollback()
		return &HTTPError{Code: 500, Message: fmt.Sprintf("Error processing line item: %v", sharedErr.err)}
	}

	for _, item := range order.LineItems {
		order.SubTotal = order.SubTotal + item.Price*item.Quantity
		if err := tx.Create(&item).Error; err != nil {
			tx.Rollback()
			return &HTTPError{Code: 500, Message: fmt.Sprintf("Error creating line item: %v", err)}
		}
	}

	return nil
}

func (a *API) processAddress(tx *gorm.DB, order *models.Order, address *models.Address, id string) (*models.Address, *HTTPError) {
	if address == nil && id == "" {
		return nil, nil
	}

	if id != "" {
		loadedAddress := new(models.Address)
		if result := tx.First(loadedAddress, "id = ?", id); result.Error != nil {
			return nil, httpError(400, "Bad address id: %v, %v", id, result.Error)
		}

		if order.UserID != loadedAddress.UserID {
			return nil, httpError(400, "Can't update the order to an address that doesn't belong to the user")
		}

		if loadedAddress.UserID == "" {
			loadedAddress.UserID = order.UserID
			tx.Save(loadedAddress)
		}
		return loadedAddress, nil
	}

	// it is a new address we're  making
	if err := address.Validate(); err != nil {
		return nil, httpError(400, "Failed to validate address: "+err.Error())
	}

	// is a valid id that doesn't already belong to a user
	address.ID = uuid.NewRandom().String()
	tx.Create(address)
	return address, nil
}

func (a *API) processLineItem(ctx context.Context, order *models.Order, item *models.LineItem) error {
	config := getConfig(ctx)
	resp, err := a.httpClient.Get(config.SiteURL + item.Path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return err
	}

	metaTag := doc.Find("#netlify-commerce-product").First()
	if metaTag.Length() == 0 {
		metaTag = doc.Find("#gocommerce-product").First() // Keep the code backwards compatible

		if metaTag.Length() == 0 {
			return fmt.Errorf("No script tag with id netlify-commerce-product tag found for '%v'", item.Title)
		}
	}
	meta := &models.LineItemMetadata{}
	err = json.Unmarshal([]byte(metaTag.Text()), meta)
	if err != nil {
		return err
	}

	return item.Process(order, meta)
}

func orderQuery(db *gorm.DB) *gorm.DB {
	return db.Preload("LineItems").Preload("ShippingAddress").Preload("BillingAddress").Preload("Data")
}
