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
	Sku      string                 `json:"sku"`
	Path     string                 `json:"path"`
	Quantity uint64                 `json:"quantity"`
	Addons   []OrderAddon           `json:"addons"`
	MetaData map[string]interface{} `json:"meta"`
}

type OrderAddon struct {
	Sku string `json:"sku"`
}

type OrderParams struct {
	SessionID string `json:"session_id"`

	Email string `json:"email"`

	IP string `json:"ip"`

	ShippingAddressID string          `json:"shipping_address_id"`
	ShippingAddress   *models.Address `json:"shipping_address"`

	BillingAddressID string          `json:"billing_address_id"`
	BillingAddress   *models.Address `json:"billing_address"`

	VATNumber string `json:"vatnumber"`

	MetaData map[string]interface{} `json:"meta"`

	LineItems []*OrderLineItem `json:"line_items"`

	Currency string `json:"currency"`

	FulfillmentState string `json:"fulfillment_state"`
}

type ReceiptParams struct {
	Email string `json:"email"`
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

// ClaimOrders will look for any orders with no user id belonging to an email and claim them
func (a *API) ClaimOrders(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	log := getLogger(ctx)

	claims := getClaims(ctx)
	if claims == nil {
		unauthorizedError(w, "Claiming an order requires a token")
		log.Info("request made with no claims")
		return
	}

	if claims.Email == "" {
		badRequestError(w, "Must provide an email in the token to claim orders")
		log.Info("Claim request made with missing email")
		return
	}

	if claims.ID == "" {
		badRequestError(w, "Must provide a ID in the token to claim orders")
		log.Info("Claim request made with missing the ID")
		return
	}

	log = log.WithFields(logrus.Fields{
		"user_id":    claims.ID,
		"user_email": claims.Email,
	})

	// now find all the order associated with that email
	query := orderQuery(a.db)
	query = query.Where(&models.Order{
		UserID: "",
		Email:  claims.Email,
	})

	orders := []models.Order{}
	if res := query.Find(&orders); res.Error != nil {
		internalServerError(w, "Failed to query for orders with email: %s", claims.Email)
		log.WithError(res.Error).Warn("Failed to make query for orders")
		return
	}

	tx := a.db.Begin()

	// create the user
	user := models.User{Email: claims.Email, ID: claims.ID}
	if res := tx.FirstOrCreate(&user); res.Error != nil {
		internalServerError(w, "Failed to create user with ID %s", claims.ID)
		log.WithError(res.Error).Warnf("Failed to creat new user: %+v", user)
		tx.Rollback()
		return
	}

	for _, o := range orders {
		o.UserID = user.ID
		if res := tx.Save(&o); res.Error != nil {
			internalServerError(w, "Failed to update an order with user ID %s", user.ID)
			log.WithError(res.Error).WithField("order_id", o.ID).Warn("Failed to save order")
			tx.Rollback()
			return
		}
	}

	if rsp := tx.Commit(); rsp.Error != nil {
		internalServerError(w, "To update all the orders")
		log.WithError(rsp.Error).Warn("Failed to close transaction")
		return
	}

	log.Info("Finished updating ")
	sendJSON(w, http.StatusNoContent, "")
}

func (a *API) ResendOrderReceipt(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	id := kami.Param(ctx, "order_id")
	log := getLogger(ctx)
	claims := getClaims(ctx)

	params := &ReceiptParams{}
	jsonDecoder := json.NewDecoder(r.Body)
	err := jsonDecoder.Decode(params)
	if err != nil {
		log.WithError(err).Infof("Failed to deserialize receipt params: %s", err.Error())
		badRequestError(w, "Could not read receipt params: %v", err)
		return
	}

	order := &models.Order{}
	if result := orderQuery(a.db).Preload("Transactions").First(order, "id = ?", id); result.Error != nil {
		if result.RecordNotFound() {
			log.Debug("Requested record that doesn't exist")
			notFoundError(w, "Order not found")
		} else {
			log.WithError(result.Error).Warnf("Error while querying database: %s", result.Error.Error())
			internalServerError(w, "Error during database query: %v", result.Error)
		}
		return
	}

	if order.UserID != "" {
		if claims == nil || (order.UserID != claims.ID && !isAdmin(ctx)) {
			unauthorizedError(w, "Order History Requires Authentication")
			log.Info("request made with no claims")
			return
		}
	}

	if params.Email != "" {
		order.Email = params.Email
	}

	for _, transaction := range order.Transactions {
		if transaction.Type == models.ChargeTransactionType {
			transaction.Order = order
			a.mailer.OrderConfirmationMail(transaction)
		}
	}

	sendJSON(w, 200, map[string]string{})
}

// OrderList can query based on
//  - orders since        &from=iso8601      - default = 0
//  - orders before       &to=iso8601        - default = now
//  - sort asc or desc    &sort=[asc | desc] - default = desc
// And you can filter on
//  - fullfilment_state=pending   - only orders pending shipping
//  - payment_state=pending       - only payd orders
//  - type=book  - filter on product type

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
	userID := kami.Param(ctx, "user_id")
	if userID != "" {
		if isAdmin(ctx) {
			id = userID
			log.WithField("admin_id", claims.ID).Debugf("Making admin request for user %s by %s", id, claims.ID)
		} else {
			log.Warnf("Request for user id %s as user %s - but not an admin", userID, id)
			badRequestError(w, "Can't request user id if you're not that user, or an admin")
			return
		}
	}
	if id != "all" {
		query = query.Where("user_id = ?", id)
	}
	log.WithField("query_user_id", id).Debug("URL parsed and query perpared")

	offset, limit, err := paginate(w, r, query.Model(&models.Order{}))
	if err != nil {
		badRequestError(w, "Bad Pagination Parameters: %v", err)
		return
	}

	var orders []models.Order
	result := query.Offset(offset).Limit(limit).Find(&orders)
	if result.Error != nil {
		log.WithError(err).Warn("Error while querying database")
		internalServerError(w, "Error during database query: %v", result.Error)
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
			internalServerError(w, "Error during database query: %v", result.Error)
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
	//c.tx = tx

	order.Email = params.Email
	order.IP = r.RemoteAddr
	order.MetaData = params.MetaData
	httpError := setOrderEmail(tx, order, claims, log)
	if httpError != nil {
		log.WithError(httpError).Info("Failed to set the order email from the token")
		cleanup(tx, w, httpError)
		return
	}

	log.WithField("order_user_id", order.UserID).Debug("Successfully set the order's ID")

	shipping, httpError := a.processAddress(tx, order, "Shipping Address", params.ShippingAddress, params.ShippingAddressID)
	if httpError != nil {
		cleanup(tx, w, httpError)
		return
	}
	if shipping == nil {
		cleanup(tx, w, badRequestError(w, "Shipping Address Required"))
		return
	}
	order.ShippingAddress = *shipping
	order.ShippingAddressID = shipping.ID

	billing, httpError := a.processAddress(tx, order, "Billing Address", params.BillingAddress, params.BillingAddressID)
	if httpError != nil {
		cleanup(tx, w, httpError)
		return
	}
	if billing != nil {
		order.BillingAddress = *billing
		order.BillingAddressID = billing.ID
	} else {
		order.BillingAddress = *shipping
		order.BillingAddressID = shipping.ID
	}

	if params.VATNumber != "" {
		valid, err := vat.IsValidVAT(params.VATNumber)
		if err != nil {
			cleanup(tx, w, internalServerError(w, "Error verifying VAT number %v", err))
			return
		}
		if !valid {
			cleanup(tx, w, badRequestError(w, "Vat number %v is not valid", order.VATNumber))
			return
		}
		order.VATNumber = params.VATNumber
	}

	if httpError := a.createLineItems(ctx, tx, order, params.LineItems); httpError != nil {
		log.WithError(httpError).Error("Failed to create order line items")
		cleanup(tx, w, httpError)
		return
	}

	log.WithField("subtotal", order.SubTotal).Debug("Successfully processed all the line items")

	tx.Create(order)
	models.LogEvent(tx, r.RemoteAddr, order.UserID, order.ID, models.EventCreated, nil)
	if a.config.Webhooks.Order != "" {
		hook := models.NewHook("order", a.config.Webhooks.Order, order)
		tx.Save(hook)
	}
	tx.Commit()

	log.Infof("Successfully created order %s", order.ID)
	sendJSON(w, 201, order)
}

// OrderUpdate will allow an ADMIN only to update the details of a record
// it is also important to note that it will not let modification of an order if the
// order is no longer pending.
// Addresses can be made by posting a new one directly, OR by referencing one by ID. If
// both are provided, the one that is made by ID will win out and the other will be ignored.
// There are also blocks to changing certain fields after the state has been locked
func (a *API) OrderUpdate(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	orderID := kami.Param(ctx, "id")
	log := getLogger(ctx).WithField("order_id", orderID)
	claims := getClaims(ctx)
	changes := []string{}

	if !isAdmin(ctx) {
		log.Warn("Illegal access attempted")
		cleanup(nil, w, unauthorizedError(w, "Admin privileges are required"))
		return
	}

	orderParams := new(OrderParams)
	err := json.NewDecoder(r.Body).Decode(orderParams)
	if err != nil {
		log.WithError(err).Infof("Failed to deserialize order params: %s", err.Error())
		cleanup(nil, w, badRequestError(w, "Could not read Order Parameters: %v", err))
		return
	}

	// verify that the order exists
	existingOrder := new(models.Order)

	rsp := orderQuery(a.db).First(existingOrder, "id = ?", orderID)
	if rsp.RecordNotFound() {
		log.Warn("Update attempted to order that doesn't exist")
		cleanup(nil, w, notFoundError(w, "Failed to find order with id '%s'", orderID))
		return
	}
	if rsp.Error != nil {
		log.WithError(rsp.Error).Warnf("Failed to query for id '%s'", orderID)
		cleanup(nil, w, internalServerError(w, "Error while querying for order"))
		return
	}

	alreadyPaid := existingOrder.PaymentState == models.PaidState

	//
	// handle the simple fields
	//
	if orderParams.SessionID != "" {
		log.Debugf("Updating session id from '%s' to '%s'", existingOrder.SessionID, orderParams.SessionID)
		existingOrder.SessionID = orderParams.SessionID
		changes = append(changes, "session_id")
	}
	if orderParams.Email != "" {
		log.Debugf("Updating email from '%s' to '%s'", existingOrder.Email, orderParams.Email)
		existingOrder.Email = orderParams.Email
		changes = append(changes, "email")
	}

	if orderParams.MetaData != nil {
		existingOrder.MetaData = orderParams.MetaData
	}

	if orderParams.Currency != "" {
		if alreadyPaid {
			log.Warn("Tried to update the currency after the order has been paid")
			cleanup(nil, w, badRequestError(w, "Can't update the currency after payment has been processed"))
			return
		}
		log.Debugf("Updating currency from '%v' to '%v'", existingOrder.Currency, orderParams.Currency)
		existingOrder.Currency = orderParams.Currency
		changes = append(changes, "currency")
	}
	if orderParams.VATNumber != "" {
		if alreadyPaid {
			log.Warn("Tried to update the VAT number after the order has been paid")
			cleanup(nil, w, badRequestError(w, "Can't update the VAT number after payment has been processed"))
			return
		}

		log.Debugf("Updating vat number from '%v' to '%v'", existingOrder.VATNumber, orderParams.VATNumber)
		existingOrder.VATNumber = orderParams.VATNumber
		changes = append(changes, "vatnumber")
	}

	tx := a.db.Begin()

	//
	// handle the addresses
	//
	if orderParams.BillingAddress != nil || orderParams.BillingAddressID != "" {
		log.Debugf("Updating order's billing address")

		addr, httpErr := a.processAddress(tx, existingOrder, "Billing Address", orderParams.BillingAddress, orderParams.BillingAddressID)
		if httpErr != nil {

			log.WithError(httpErr).Warn("Failed to update the billing address")
			cleanup(tx, w, httpErr)
			return
		}
		old := existingOrder.BillingAddressID
		existingOrder.BillingAddress = *addr
		log.WithFields(logrus.Fields{
			"address_id":     addr.ID,
			"old_address_id": old,
		}).Debugf("Updated the billing address id to %s", addr.ID)
		changes = append(changes, "billing_address")
	}

	if orderParams.ShippingAddress != nil || orderParams.ShippingAddressID != "" {
		log.Debugf("Updating order's shipping address")

		addr, httpErr := a.processAddress(tx, existingOrder, "Shipping Address", orderParams.ShippingAddress, orderParams.ShippingAddressID)
		if httpErr != nil {
			log.WithError(httpErr).Warn("Failed to update the shipping address")
			cleanup(tx, w, httpErr)
			return
		}

		old := existingOrder.ShippingAddressID
		existingOrder.ShippingAddress = *addr
		log.WithFields(logrus.Fields{
			"address_id":     addr.ID,
			"old_address_id": old,
		}).Debugf("Updated the shipping address id to %s", addr.ID)
		changes = append(changes, "shipping_address")
	}

	if orderParams.FulfillmentState != "" {
		_, ok := map[string]bool{
			"pending":  true,
			"shipping": true,
			"shipped":  true,
		}[orderParams.FulfillmentState]
		if !ok {
			log.WithError(err).Warn("Failed to update order data: " + err.Error())
			cleanup(tx, w, badRequestError(w, "Bad fulfillment state: "+orderParams.FulfillmentState))
			return
		}
		existingOrder.FulfillmentState = orderParams.FulfillmentState
		changes = append(changes, "fulfillment_state")
	}

	//
	// handle the line items
	//
	updatedItems := make(map[string]*OrderLineItem)
	for _, item := range orderParams.LineItems {
		updatedItems[item.Sku] = item
	}

	for _, item := range existingOrder.LineItems {
		if update, exists := updatedItems[item.Sku]; exists {
			item.Quantity = update.Quantity
			if update.Path != "" {
				item.Path = update.Path
			}
		}
	}

	if len(updatedItems) > 0 {
		changes = append(changes, "line_items")
	}

	log.Info("Saving order updates")
	if rsp := tx.Save(existingOrder); rsp.Error != nil {
		log.WithError(err).Warn("Problem while saving order updates")
		cleanup(tx, w, internalServerError(w, "Error saving order updates"))
		return
	}

	models.LogEvent(tx, r.RemoteAddr, claims.ID, existingOrder.ID, models.EventUpdated, changes)
	if rsp := tx.Commit(); rsp.Error != nil {
		log.WithError(err).Warn("Problem while committing order updates")
		cleanup(tx, w, internalServerError(w, "Error committing order updates"))
		return
	}

	sendJSON(w, 200, existingOrder)
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
	for _, orderItem := range items {
		lineItem := &models.LineItem{
			Sku:      orderItem.Sku,
			Quantity: orderItem.Quantity,
			MetaData: orderItem.MetaData,
			Path:     orderItem.Path,
			OrderID:  order.ID,
		}
		order.LineItems = append(order.LineItems, lineItem)
		sem <- 1
		wg.Add(1)
		go func(item *models.LineItem, orderItem *OrderLineItem) {
			defer func() {
				wg.Done()
				<-sem
			}()
			// Stop doing any work if there's already an error
			if sharedErr.err != nil {
				return
			}

			if err := a.processLineItem(ctx, order, item, orderItem); err != nil {
				sharedErr.setError(err)
			}
		}(lineItem, orderItem)
	}
	wg.Wait()

	if sharedErr.err != nil {
		return &HTTPError{Code: 500, Message: fmt.Sprintf("Error processing line item: %v", sharedErr.err)}
	}

	for _, item := range order.LineItems {
		order.SubTotal = order.SubTotal + (item.Price+item.AddonPrice)*item.Quantity
		if err := tx.Save(&item).Error; err != nil {
			return &HTTPError{Code: 500, Message: fmt.Sprintf("Error creating line item: %v", err)}
		}
	}

	for _, download := range order.Downloads {
		if err := tx.Create(&download).Error; err != nil {
			return &HTTPError{Code: 500, Message: fmt.Sprintf("Error creating download item: %v", err)}
		}
	}

	settings, err := a.loadSettings(ctx)
	if err != nil {
		return &HTTPError{Code: 500, Message: err.Error()}
	}

	order.CalculateTotal(settings)

	return nil
}

func (a *API) loadSettings(ctx context.Context) (*models.SiteSettings, error) {
	config := getConfig(ctx)

	settings := &models.SiteSettings{}
	resp, err := a.httpClient.Get(config.SiteURL + "/netlify-commerce/settings.json")
	if err != nil {
		return nil, fmt.Errorf("Error loading site settings: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(settings); err != nil {
			return nil, fmt.Errorf("Error parsing site settings: %v", err)
		}
	}

	return settings, nil
}

func (a *API) processAddress(tx *gorm.DB, order *models.Order, name string, address *models.Address, id string) (*models.Address, *HTTPError) {
	if address == nil && id == "" {
		return nil, nil
	}

	if id != "" {
		loadedAddress := new(models.Address)
		if result := tx.First(loadedAddress, "id = ?", id); result.Error != nil {
			return nil, httpError(400, "Bad %v id: %v, %v", name, id, result.Error)
		}

		if order.UserID != loadedAddress.UserID {
			return nil, httpError(400, "Can't update the order to an %v that doesn't belong to the user", name)
		}

		if loadedAddress.UserID == "" {
			loadedAddress.UserID = order.UserID
			tx.Save(loadedAddress)
		}
		return loadedAddress, nil
	}

	// it is a new address we're  making
	if err := address.Validate(); err != nil {
		return nil, httpError(400, "Failed to validate %v: %v", name, err.Error())
	}

	// is a valid id that doesn't already belong to a user
	address.ID = uuid.NewRandom().String()
	tx.Create(address)
	return address, nil
}

func (a *API) processLineItem(ctx context.Context, order *models.Order, item *models.LineItem, orderItem *OrderLineItem) error {
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

	metaTag := doc.Find(".netlify-commerce-product")
	if metaTag.Length() == 0 {
		return fmt.Errorf("No script tag with id netlify-commerce-product tag found for '%v'", item.Title)
	}
	metaProducts := []*models.LineItemMetadata{}
	var parsingErr error
	metaTag.Each(func(_ int, tag *goquery.Selection) {
		if parsingErr != nil {
			return
		}
		meta := &models.LineItemMetadata{}
		parsingErr = json.Unmarshal([]byte(tag.Text()), meta)
		if parsingErr == nil {
			metaProducts = append(metaProducts, meta)
		}
	})
	if parsingErr != nil {
		return fmt.Errorf("Error parsing product metadata: %v", parsingErr)
	}

	if len(metaProducts) == 1 && item.Sku == "" {
		item.Sku = metaProducts[0].Sku
	}

	for _, meta := range metaProducts {
		if meta.Sku == item.Sku {
			for _, addon := range orderItem.Addons {
				item.AddonItems = append(item.AddonItems, models.AddonItem{
					Sku: addon.Sku,
				})
			}

			return item.Process(order, meta)
		}
	}

	return fmt.Errorf("No product Sku from path matched: %v", item.Sku)
}

func orderQuery(db *gorm.DB) *gorm.DB {
	return db.
		Preload("LineItems").
		Preload("Downloads").
		Preload("ShippingAddress").
		Preload("BillingAddress").
		Preload("Transactions")
}

func cleanup(tx *gorm.DB, w http.ResponseWriter, e *HTTPError) {
	if e != nil {
		sendJSON(w, e.Code, e)
		if tx != nil {
			tx.Rollback()
		}
	}
}
