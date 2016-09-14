package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/Sirupsen/logrus"
	"github.com/guregu/kami"
	"github.com/jinzhu/gorm"
	"github.com/mattes/vat"
	"github.com/netlify/gocommerce/models"
	"github.com/pborman/uuid"

	"golang.org/x/net/context"
)

// OrderLineItem describe a single item that is being ordered
type OrderLineItem struct {
	SKU      string `json:sku`
	Path     string `json:"path"`
	Quantity uint64 `json:"quantity"`
}

// OrderParams describe the order that is being placed
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

// OrderList can query based on
//  - orders since        &from=iso8601      - default = 0
//  - orders before       &to=iso8601        - default = now
//  - sort asc or desc    &sort=[asc | desc] - default = desc
func (a *API) OrderList(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	log := Logger(ctx)
	var err error
	claims := Claims(ctx)
	if claims == nil {
		log.Info("Request with no claims made")
		UnauthorizedError(w, "Order History Requires Authentication")
		return
	}

	params := r.URL.Query()
	query := orderQuery(a.db)
	query, err = parseParams(query, params)
	if err != nil {
		log.WithError(err).Info("Bad query parameters in request")
		BadRequestError(w, "Bad parameters in query: "+err.Error())
		return
	}

	// handle the admin info
	id := claims.ID
	if values, exists := params["user_id"]; exists {
		if IsAdmin(ctx) {
			id = values[0]
			log.WithField("admin_id", claims.ID).Debugf("Making admin request for user %s by %s", id, claims.ID)
		} else {
			log.Warnf("Request for user id %s as user %s - but not an admin", values[0], id)
			BadRequestError(w, "Can't request user id if you're not that user, or an admin")
			return
		}
	}
	query = query.Where("user_id = ?", id)
	log.WithField("query_user_id", id).Debug("URL parsed and query perpared")

	var orders []models.Order
	result := query.Find(&orders)
	if result.Error != nil {
		log.WithError(err).Warn("Error while querying database")
		InternalServerError(w, fmt.Sprintf("Error during database query: %v", result.Error))
		return
	}

	log.WithField("order_count", len(orders)).Debugf("Successfully retrieved %d orders", len(orders))
	sendJSON(w, 200, orders)
}

// OrderView will request a specific order using the 'id' parameter.
// Only the owner of the order, an admin, or an anon order are allowed to be seen
func (a *API) OrderView(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	log := Logger(ctx)
	claims := Claims(ctx)
	if claims == nil {
		log.Info("Request with no claims made")
		UnauthorizedError(w, "Order History Requires Authentication")
		return
	}

	id := kami.Param(ctx, "id")
	if id == "" {
		log.Warn("Request made with no id parameter")
		BadRequestError(w, "Must pass an id parameter")
		return
	}
	log = log.WithField("order_id", id)

	order := &models.Order{}
	if result := orderQuery(a.db).First(order, "id = ?", id); result.Error != nil {
		if result.RecordNotFound() {
			log.Debug("Requested record that doesn't exist")
			NotFoundError(w, "Order not found")
		} else {
			log.WithError(result.Error).Warnf("Error while querying database: %s", result.Error.Error())
			InternalServerError(w, "Error during database query")
		}
		return
	}

	if order.UserID == "" || (order.UserID == claims.ID) || IsAdmin(ctx) {
		log.Debugf("Successfully got order %s", order.ID)
		sendJSON(w, 200, order)
	} else {
		log.WithFields(logrus.Fields{
			"user_id":     claims.ID,
			"user_groups": claims.Groups,
		}).Warnf("Unauthorized access attempted for order %s by %s", order.ID, claims.ID)
		UnauthorizedError(w, "You don't have access to this order")
	}
}

type closer struct {
	w         http.ResponseWriter
	httpError *HTTPError
	tx        *gorm.DB
}

func (c closer) close() {
	if c.httpError != nil {
		sendJSON(c.w, c.httpError.Code, c.httpError)
		if c.tx != nil {
			c.tx.Rollback()
		}
	}
}

// OrderUpdate will allow an ADMIN only to update the details of a record
// it is also important to note that it will not let modification of an order if the
// order is no longer pending.
// Addresses can be made by posting a new one directly, OR by referencing one by ID. If
// both are provided, the one that is made by ID will win out and the other will be ignored.
// There are also blocks to changing certain fields after the state has been locked
func (a *API) OrderUpdate(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	orderID := kami.Param(ctx, "id")
	log := Logger(ctx).WithField("order_id", orderID)
	cl := closer{w: w}
	defer cl.close()

	if !IsAdmin(ctx) {
		log.Warn("Illegal access attempted")
		cl.httpError = UnauthorizedError(w, "Admin privileges are required")
		return
	}

	orderParams := new(OrderParams)
	err := json.NewDecoder(r.Body).Decode(orderParams)
	if err != nil {
		log.WithError(err).Infof("Failed to deserialize order params: %s", err.Error())
		cl.httpError = BadRequestError(w, fmt.Sprintf("Could not read Order Parameters: %v", err))
		return
	}

	// verify that the order exists
	existingOrder := new(models.Order)

	rsp := orderQuery(a.db).First(existingOrder, "id = ?", orderID)
	if rsp.RecordNotFound() {
		log.Warn("Update attempted to order that doesn't exist")
		cl.httpError = NotFoundError(w, fmt.Sprintf("Failed to find order with id '%s'", orderID))
		return
	}
	if rsp.Error != nil {
		log.WithError(rsp.Error).Warnf("Failed to query for id '%s'", orderID)
		cl.httpError = InternalServerError(w, "Error while querying for order")
		return
	}

	if existingOrder.State != models.PendingState {
		log.Warn("Tried to update order that is no longer pending")
		cl.httpError = BadRequestError(w, "Order is no longer pending - can't update details")
		return
	}

	alreadyPaid := existingOrder.PaymentState == models.PaidState
	alreadyShipped := existingOrder.FulfillmentState == models.PaidState

	//
	// handle the simple fields
	//
	if orderParams.SessionID != "" {
		log.Debugf("Updating session id from '%s' to '%s'", existingOrder.SessionID, orderParams.SessionID)
		existingOrder.SessionID = orderParams.SessionID
	}
	if orderParams.Email != "" {
		log.Debugf("Updating email from '%s' to '%s'", existingOrder.Email, orderParams.Email)
		existingOrder.Email = orderParams.Email
	}

	if orderParams.Currency != "" {
		if alreadyPaid {
			log.Warn("Tried to update the currency after the order has been paid")
			cl.httpError = BadRequestError(w, "Can't update the currency after payment has been processed")
			return
		}
		log.Debugf("Updating currency from '%v' to '%v'", existingOrder.Currency, orderParams.Currency)
		existingOrder.Currency = orderParams.Currency

	}
	if orderParams.VATNumber != "" {
		if alreadyPaid {
			log.Warn("Tried to update the VAT number after the order has been paid")
			cl.httpError = BadRequestError(w, "Can't update the VAT number after payment has been processed")
			return
		}

		log.Debugf("Updating vat number from '%v' to '%v'", existingOrder.VATNumber, orderParams.VATNumber)
		existingOrder.VATNumber = orderParams.VATNumber
	}

	tx := a.db.Begin()
	cl.tx = tx

	//
	// handle the addresses
	//
	if orderParams.BillingAddress != nil || orderParams.BillingAddressID != "" {
		if alreadyPaid {
			log.Warn("Tried to update the billing address after payment")
			cl.httpError = BadRequestError(w, "Can't update the billing address of an order that has already been paid")
			return
		}
		log.Debugf("Updating order's billing address")

		addr, httpErr := a.processAddress(tx, existingOrder, orderParams.BillingAddress, orderParams.BillingAddressID)
		if httpErr != nil {
			log.WithError(httpErr).Warn("Failed to update the billing address")
			cl.httpError = httpErr
			return
		}
		old := existingOrder.BillingAddressID
		existingOrder.BillingAddress = *addr
		log.WithFields(logrus.Fields{
			"address_id":     addr.ID,
			"old_address_id": old,
		}).Debugf("Updated the billing address id to %s", addr.ID)
	}

	if orderParams.ShippingAddress != nil || orderParams.ShippingAddressID != "" {
		if alreadyShipped {
			log.Warn("Tried to update the shipping address after it has shipped")
			cl.httpError = BadRequestError(w, "Can't update the shipping address of an order that has been shipped")
			return
		}

		log.Debugf("Updating order's shipping address")

		addr, httpErr := a.processAddress(tx, existingOrder, orderParams.ShippingAddress, orderParams.ShippingAddressID)
		if httpErr != nil {
			log.WithError(httpErr).Warn("Failed to update the shipping address")
			cl.httpError = httpErr
			return
		}

		old := existingOrder.ShippingAddressID
		existingOrder.ShippingAddress = *addr
		log.WithFields(logrus.Fields{
			"address_id":     addr.ID,
			"old_address_id": old,
		}).Debugf("Updated the shipping address id to %s", addr.ID)
	}

	if orderParams.Data != nil {
		err := existingOrder.UpdateOrderData(tx, &orderParams.Data)
		if err != nil {
			log.WithError(err).Warn("Failed to update order data: " + err.Error())
			if _, ok := err.(*models.InvalidDataType); ok {
				cl.httpError = BadRequestError(w, "Bad type: "+err.Error())
			} else {
				cl.httpError = InternalServerError(w, "Problem while saving the extra data")
			}
			return
		}
	}

	//
	// handle the line items
	//
	updatedItems := make(map[string]*OrderLineItem)
	for _, item := range orderParams.LineItems {
		updatedItems[item.SKU] = item
	}

	for _, item := range existingOrder.LineItems {
		if update, exists := updatedItems[item.SKU]; exists {
			item.Quantity = update.Quantity
			if update.Path != "" {
				item.Path = update.Path
			}
		}
	}

	log.Info("Saving order updates")
	if rsp := tx.Save(existingOrder); rsp.Error != nil {
		log.WithError(err).Warn("Problem while saving order updates")
		cl.httpError = InternalServerError(w, "Error saving order updates")
	}
	if rsp := tx.Commit(); rsp.Error != nil {
		log.WithError(err).Warn("Problem while saving order updates")
		cl.httpError = InternalServerError(w, "Error saving order updates")
	}

	sendJSON(w, 200, existingOrder)
}

// OrderCreate endpoint for creating an order. It does NOT require a token because
// it is possible to create an anonymous order. There will be no user for that order.
func (a *API) OrderCreate(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	log := Logger(ctx)

	params := &OrderParams{Currency: "USD"}
	jsonDecoder := json.NewDecoder(r.Body)
	err := jsonDecoder.Decode(params)
	if err != nil {
		log.WithError(err).Infof("Failed to deserialize order params: %s", err.Error())
		BadRequestError(w, fmt.Sprintf("Could not read Order params: %v", err))
		return
	}

	claims := Claims(ctx)

	order := models.NewOrder(params.SessionID, params.Email, params.Currency)
	log = log.WithFields(logrus.Fields{
		"order_id":   order.ID,
		"session_id": params.SessionID,
	})
	ctx = WithLogger(ctx, log)

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
		BadRequestError(w, "Shipping Address Required")
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
			InternalServerError(w, fmt.Sprintf("Error verifying VAT number %v", err))
			return
		}
		if !valid {
			tx.Rollback()
			BadRequestError(w, fmt.Sprintf("Vat number %v is not valid", order.VATNumber))
			return
		}
		order.VATNumber = params.VATNumber
	}

	if params.Data != nil {
		if err := order.UpdateOrderData(tx, &params.Data); err != nil {
			tx.Rollback()
			BadRequestError(w, fmt.Sprintf("Bad order metadata %v", err))
			return
		}
	}

	tx.Create(order)
	tx.Commit()

	log.Infof("Successfully created order %s", order.ID)
	sendJSON(w, 201, order)
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
	siteURL := Config(ctx).SiteURL
	log := Logger(ctx).WithField("site_url", siteURL)

	var wg sync.WaitGroup
	sem := make(chan int, MaxConcurrentLookups)
	sharedErr := sharedError{}

	for _, item := range items {
		lineItem := &models.LineItem{
			SKU:      item.SKU,
			Quantity: item.Quantity,
			Path:     item.Path,
			OrderID:  order.ID,
		}
		order.LineItems = append(order.LineItems, lineItem)

		sem <- 1
		wg.Add(1)
		go func(item *models.LineItem) {
			itemLog := log.WithField("item_id", item.ID)
			itemLog.WithFields(logrus.Fields{
				"price":    item.Price,
				"sku":      item.SKU,
				"path":     item.Path,
				"quantity": item.Quantity,
			}).Debug("Starting to process item")
			defer func() {
				itemLog.Debug("Completed processing item")
				wg.Done()
				<-sem
			}()

			// Stop doing any work if there's already an error
			if sharedErr.hasError() {
				itemLog.Debug("Skipping item because of previous error")
				return
			}

			if err := a.processLineItem(order, item, siteURL, itemLog); err != nil {
				itemLog.WithError(err).Debug("Error while processing item")
				sharedErr.setError(err)
			}
		}(lineItem)
	}

	log.Debugf("Waiting to process %d line items", len(items))
	wg.Wait()
	log.Debug("Finished processing line items")

	if sharedErr.hasError() {
		return httpError(500, "Error processing line item: %v", sharedErr.err)
	}

	for _, item := range order.LineItems {
		order.SubTotal = order.SubTotal + item.Price*item.Quantity
		if err := tx.Create(&item).Error; err != nil {
			return httpError(500, "Error creating line item: %v", err)
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
	if !address.Valid() {
		return nil, httpError(400, "Failed to validate address")
	}

	// is a valid id that doesn't already belong to a user
	address.ID = uuid.NewRandom().String()
	tx.Create(address)

	return address, nil
}

func (a *API) processLineItem(order *models.Order, item *models.LineItem, siteURL string, log *logrus.Entry) error {
	endpoint := siteURL + item.Path
	resp, err := a.httpClient.Get(endpoint)
	if err != nil {
		log.WithError(err).Warnf("Failed make the request for the item at %s", endpoint)
		return err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		log.WithError(err).Warnf("Failed to parse the response into a document")
		return err
	}

	metaTag := doc.Find("#gocommerce-product").First()
	if metaTag.Length() == 0 {
		return fmt.Errorf("No script tag with id gocommerce-product tag found for '%v'", item.Title)
	}

	meta := &models.LineItemMetadata{}
	err = json.Unmarshal([]byte(metaTag.Text()), meta)
	if err != nil {
		log.WithError(err).Warn("Failed to unmarshal the item's metadata")
		return err
	}

	return item.Process(order, meta)
}

func orderQuery(db *gorm.DB) *gorm.DB {
	return db.Preload("LineItems").Preload("ShippingAddress").Preload("BillingAddress").Preload("Data")
}

func parseParams(query *gorm.DB, params url.Values) (*gorm.DB, error) {
	if values, exists := params["from"]; exists {
		date, err := time.Parse(time.RFC3339, values[0])
		if err != nil {
			return nil, fmt.Errorf("bad value for 'from' parameter: %s", err)
		}
		query = query.Where("created_at >= ?", date)
	}

	if values, exists := params["to"]; exists {
		date, err := time.Parse(time.RFC3339, values[0])
		if err != nil {
			return nil, fmt.Errorf("bad value for 'to' parameter: %s", err)
		}
		query = query.Where("created_at <= ?", date)
	}

	if values, exists := params["sort"]; exists {
		switch values[0] {
		case "desc":
			query = query.Order("created_at DESC")
		case "asc":
			query = query.Order("created_at ASC")
		default:
			return nil, fmt.Errorf("bad value for 'sort' parameter: only 'asc' or 'desc' are accepted")
		}
	}

	return query, nil
}

func (a *API) checkExistence(inter interface{}) bool {
	return !a.db.First(inter).RecordNotFound()
}
