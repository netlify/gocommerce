package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-chi/chi"
	"github.com/jinzhu/gorm"
	"github.com/mattes/vat"
	"github.com/netlify/gocommerce/calculator"
	"github.com/netlify/gocommerce/claims"
	gcontext "github.com/netlify/gocommerce/context"
	"github.com/netlify/gocommerce/models"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
)

// MaxConcurrentLookups controls the number of simultaneous HTTP Order lookups
const MaxConcurrentLookups = 10

type orderLineItem struct {
	Sku      string                 `json:"sku"`
	Path     string                 `json:"path"`
	Quantity uint64                 `json:"quantity"`
	Addons   []orderAddon           `json:"addons"`
	MetaData map[string]interface{} `json:"meta"`
}

type orderAddon struct {
	Sku string `json:"sku"`
}

type orderRequestParams struct {
	SessionID string `json:"session_id"`

	Email string `json:"email"`

	IP string `json:"ip"`

	ShippingAddressID string          `json:"shipping_address_id"`
	ShippingAddress   *models.Address `json:"shipping_address"`

	BillingAddressID string          `json:"billing_address_id"`
	BillingAddress   *models.Address `json:"billing_address"`

	VATNumber string `json:"vatnumber"`

	MetaData map[string]interface{} `json:"meta"`

	LineItems []*orderLineItem `json:"line_items"`

	Currency string `json:"currency"`

	FulfillmentState string `json:"fulfillment_state"`

	CouponCode string `json:"coupon"`
}

type receiptParams struct {
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

func (a *API) withOrderID(w http.ResponseWriter, r *http.Request) (context.Context, error) {
	orderID := chi.URLParam(r, "order_id")
	logEntrySetField(r, "order_id", orderID)

	ctx := gcontext.WithOrderID(r.Context(), orderID)
	return ctx, nil
}

// ClaimOrders will look for any orders with no user id belonging to an email and claim them
func (a *API) ClaimOrders(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := getLogEntry(r)
	instanceID := gcontext.GetInstanceID(ctx)

	claims := gcontext.GetClaims(ctx)
	if claims.Email == "" {
		return badRequestError("Must provide an email in the token to claim orders")
	}

	if claims.Subject == "" {
		return badRequestError("Must provide a ID in the token to claim orders")
	}

	log = log.WithFields(logrus.Fields{
		"user_id":    claims.Subject,
		"user_email": claims.Email,
	})

	// now find all the order associated with that email
	query := orderQuery(a.db)
	query = query.Where(&models.Order{
		InstanceID: instanceID,
		UserID:     "",
		Email:      claims.Email,
	})

	orders := []models.Order{}
	if res := query.Find(&orders); res.Error != nil {
		return internalServerError("Failed to query for orders with email: %s", claims.Email).WithInternalError(res.Error)
	}

	tx := a.db.Begin()

	// create the user
	user := models.User{
		InstanceID: instanceID,
		ID:         claims.Subject,
		Email:      claims.Email,
	}
	if res := tx.FirstOrCreate(&user); res.Error != nil {
		tx.Rollback()
		return internalServerError("Failed to create user with ID %s", claims.Subject).WithInternalError(res.Error).WithInternalMessage("Failed to create new user: %+v", user)
	}

	for _, o := range orders {
		o.UserID = user.ID
		o.BillingAddress.UserID = user.ID
		o.ShippingAddress.UserID = user.ID

		if res := tx.Save(&o); res.Error != nil {
			tx.Rollback()
			return internalServerError("Failed to update an order with user ID %s", user.ID).WithInternalError(res.Error).WithInternalMessage("Failed to update order ID %s", o.ID)
		}
	}

	if rsp := tx.Commit(); rsp.Error != nil {
		return internalServerError("Failed to update all the orders").WithInternalError(rsp.Error)
	}

	log.Info("Finished updating")
	return sendJSON(w, http.StatusNoContent, "")
}

// ReceiptView renders an HTML receipt for an order
func (a *API) ReceiptView(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	id := gcontext.GetOrderID(ctx)
	logEntrySetField(r, "order_id", id)

	order := &models.Order{}
	if result := orderQuery(a.db).Preload("Transactions").First(order, "id = ?", id); result.Error != nil {
		if result.RecordNotFound() {
			return notFoundError("Order not found")
		}
		return internalServerError("Error during database query").WithInternalError(result.Error)
	}

	if !hasOrderAccess(ctx, order) {
		return unauthorizedError("Order History Requires Authentication")
	}
	template := r.URL.Query().Get("template")

	mailer := gcontext.GetMailer(ctx)
	for _, transaction := range order.Transactions {
		if transaction.Type == models.ChargeTransactionType {
			transaction.Order = order
			html, err := mailer.OrderConfirmationMailBody(transaction, template)
			if err != nil {
				return internalServerError("Error creating receipt").WithInternalError(err)
			}
			w.WriteHeader(http.StatusOK)
			w.Header().Add("Content-Type", "text/html")
			w.Write([]byte(html))
			return nil
		}
	}

	return notFoundError("Receipt not found")
}

// ResendOrderReceipt resends the email receipt for an order
func (a *API) ResendOrderReceipt(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	id := gcontext.GetOrderID(ctx)
	log := getLogEntry(r)

	params := &receiptParams{}
	jsonDecoder := json.NewDecoder(r.Body)
	err := jsonDecoder.Decode(params)
	if err != nil {
		return badRequestError("Could not read receipt params: %v", err)
	}

	order := &models.Order{}
	if result := orderQuery(a.db).Preload("Transactions").First(order, "id = ?", id); result.Error != nil {
		if result.RecordNotFound() {
			return notFoundError("Order not found")
		}
		return internalServerError("Error during database query").WithInternalError(result.Error)
	}

	if !hasOrderAccess(ctx, order) {
		return unauthorizedError("Order History Requires Authentication")
	}

	if params.Email != "" {
		order.Email = params.Email
	}

	mailer := gcontext.GetMailer(ctx)
	for _, transaction := range order.Transactions {
		if transaction.Type == models.ChargeTransactionType {
			transaction.Order = order
			if mailErr := mailer.OrderConfirmationMail(transaction); mailErr != nil {
				log.WithError(mailErr).Errorf("Error sending order confirmation mail")
			}
		}
	}

	return sendJSON(w, http.StatusOK, map[string]string{})
}

// OrderList can query based on
//  - orders since        &from=iso8601      - default = 0
//  - orders before       &to=iso8601        - default = now
//  - sort asc or desc    &sort=[asc | desc] - default = desc
// And you can filter on
//  - fullfilment_state=pending   - only orders pending shipping
//  - payment_state=pending       - only paid orders
//  - type=book  - filter on product type
//  - email
//  - items

// OrderList lists orders selected by the query parameters provided.
func (a *API) OrderList(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := getLogEntry(r)
	claims := gcontext.GetClaims(ctx)
	instanceID := gcontext.GetInstanceID(ctx)

	var err error
	params := r.URL.Query()
	query := orderQuery(a.db)
	query, err = parseOrderParams(query, params)
	if err != nil {
		return badRequestError("Bad parameters in query: %v", err)
	}
	query = query.Where("instance_id = ?", instanceID)

	userID := gcontext.GetUserID(ctx)
	if userID == "" {
		userID = claims.Subject
	}
	if userID != "all" {
		orderTable := query.NewScope(models.Order{}).QuotedTableName()
		query = query.Where(orderTable+".user_id = ?", userID)
	}
	log.WithField("query_user_id", userID).Debug("URL parsed and query perpared")

	offset, limit, err := paginate(w, r, query.Model(&models.Order{}))
	if err != nil {
		return badRequestError("Bad Pagination Parameters: %v", err)
	}

	var orders []models.Order
	result := query.Offset(offset).Limit(limit).Find(&orders)
	if result.Error != nil {
		return internalServerError("Error during database query").WithInternalError(result.Error)
	}

	log.WithField("order_count", len(orders)).Debugf("Successfully retrieved %d orders", len(orders))
	return sendJSON(w, http.StatusOK, orders)
}

// OrderView will request a specific order using the 'id' parameter.
// Only the owner of the order, an admin, or an anon order are allowed to be seen
func (a *API) OrderView(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	id := gcontext.GetOrderID(ctx)
	log := getLogEntry(r)

	order := &models.Order{}
	if result := orderQuery(a.db).First(order, "id = ?", id); result.Error != nil {
		if result.RecordNotFound() {
			return notFoundError("Order not found")
		}
		return internalServerError("Error during database query").WithInternalError(result.Error)
	}

	if !hasOrderAccess(ctx, order) {
		return unauthorizedError("You don't have access to this order")
	}

	log.Debugf("Successfully got order %s", order.ID)
	return sendJSON(w, http.StatusOK, order)
}

// OrderCreate endpoint
func (a *API) OrderCreate(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	config := gcontext.GetConfig(ctx)
	instanceID := gcontext.GetInstanceID(ctx)

	params := &orderRequestParams{Currency: "USD"}
	jsonDecoder := json.NewDecoder(r.Body)
	err := jsonDecoder.Decode(params)
	if err != nil {
		return badRequestError("Could not read Order params: %v", err)
	}

	claims := gcontext.GetClaims(ctx)
	order := models.NewOrder(instanceID, params.SessionID, params.Email, params.Currency)

	if params.CouponCode != "" {
		coupon, err := a.lookupCoupon(ctx, w, params.CouponCode)
		if err != nil {
			return err
		}
		if !coupon.Valid() {
			return badRequestError("This coupon is not valid at this time")
		}

		order.CouponCode = coupon.Code
		order.Coupon = coupon
	}

	log := logEntrySetFields(r, logrus.Fields{
		"order_id":   order.ID,
		"session_id": params.SessionID,
	})
	log.WithFields(logrus.Fields{
		"email":    params.Email,
		"currency": params.Currency,
	}).Debug("Created order, starting to process request")
	tx := a.db.Begin()

	order.IP = r.RemoteAddr
	order.MetaData = params.MetaData
	httpError := setOrderEmail(tx, order, claims, log)
	if httpError != nil {
		log.WithError(httpError).Info("Failed to set the order email from the token")
		tx.Rollback()
		return httpError
	}

	log.WithField("order_user_id", order.UserID).Debug("Successfully set the order's ID")

	shipping, httpError := a.processAddress(tx, order, "Shipping Address", params.ShippingAddress, params.ShippingAddressID)
	if httpError != nil {
		tx.Rollback()
		return httpError
	}
	if shipping == nil {
		tx.Rollback()
		return badRequestError("Shipping Address Required")
	}
	order.ShippingAddress = *shipping
	order.ShippingAddressID = shipping.ID

	billing, httpError := a.processAddress(tx, order, "Billing Address", params.BillingAddress, params.BillingAddressID)
	if httpError != nil {
		tx.Rollback()
		return httpError
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
			tx.Rollback()
			return internalServerError("Error verifying VAT number").WithInternalError(err)
		}
		if !valid {
			tx.Rollback()
			return badRequestError("Vat number %v is not valid", order.VATNumber)
		}
		order.VATNumber = params.VATNumber
	}

	if httpError := a.createLineItems(ctx, tx, order, params.LineItems, log); httpError != nil {
		log.WithError(httpError).Error("Failed to create order line items")
		tx.Rollback()
		return httpError
	}

	log.WithField("subtotal", order.SubTotal).Debug("Successfully processed all the line items")

	tx.Create(order)
	models.LogEvent(tx, r.RemoteAddr, order.UserID, order.ID, models.EventCreated, nil)
	if config.Webhooks.Order != "" {
		hook, err := models.NewHook("order", config.SiteURL, config.Webhooks.Order, order.UserID, config.Webhooks.Secret, order)
		if err != nil {
			log.WithError(err).Error("Failed to process webhook")
		}
		tx.Save(hook)
	}
	tx.Commit()

	log.Infof("Successfully created order %s", order.ID)
	return sendJSON(w, http.StatusCreated, order)
}

// OrderUpdate will allow an ADMIN only to update the details of a record
// it is also important to note that it will not let modification of an order if the
// order is no longer pending.
// Addresses can be made by posting a new one directly, OR by referencing one by ID. If
// both are provided, the one that is made by ID will win out and the other will be ignored.
// There are also blocks to changing certain fields after the state has been locked
func (a *API) OrderUpdate(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	orderID := gcontext.GetOrderID(ctx)
	log := getLogEntry(r)
	claims := gcontext.GetClaims(ctx)
	config := gcontext.GetConfig(ctx)
	changes := []string{}

	orderParams := new(orderRequestParams)
	err := json.NewDecoder(r.Body).Decode(orderParams)
	if err != nil {
		return badRequestError("Could not read Order Parameters: %v", err)
	}

	// verify that the order exists
	existingOrder := new(models.Order)

	rsp := orderQuery(a.db).First(existingOrder, "id = ?", orderID)
	if rsp.RecordNotFound() {
		return notFoundError("Failed to find order with id '%s'", orderID)
	}
	if rsp.Error != nil {
		return internalServerError("Error while querying for order").WithInternalError(rsp.Error)
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
			return badRequestError("Can't update the currency after payment has been processed")
		}
		log.Debugf("Updating currency from '%v' to '%v'", existingOrder.Currency, orderParams.Currency)
		existingOrder.Currency = orderParams.Currency
		changes = append(changes, "currency")
	}
	if orderParams.VATNumber != "" {
		if alreadyPaid {
			return badRequestError("Can't update the VAT number after payment has been processed")
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
			tx.Rollback()
			return httpErr
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
			tx.Rollback()
			return httpErr
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
			tx.Rollback()
			return badRequestError("Bad fulfillment state: " + orderParams.FulfillmentState)
		}
		existingOrder.FulfillmentState = orderParams.FulfillmentState
		changes = append(changes, "fulfillment_state")
	}

	//
	// handle the line items
	//
	updatedItems := make(map[string]*orderLineItem)
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
		tx.Rollback()
		return internalServerError("Error saving order updates").WithInternalError(rsp.Error)
	}

	models.LogEvent(tx, r.RemoteAddr, claims.Subject, existingOrder.ID, models.EventUpdated, changes)
	if config.Webhooks.Update != "" {
		// TODO should this be claims.Subject or existingOrder.UserID ?
		hook, err := models.NewHook("update", config.SiteURL, config.Webhooks.Update, claims.Subject, config.Webhooks.Secret, existingOrder)
		if err != nil {
			log.WithError(err).Error("Failed to process web hook")
		}
		tx.Save(hook)
	}
	if rsp := tx.Commit(); rsp.Error != nil {
		tx.Rollback()
		return internalServerError("Error committing order updates").WithInternalError(rsp.Error)
	}

	return sendJSON(w, http.StatusOK, existingOrder)
}

// An order's email is determined by a few things. The rules guiding it are:
// 1 - if no claims are provided then the one in the params is used (for anon orders)
// 2 - if claims are provided they must be a valid user id
// 3 - if that user doesn't exist then a user will be created with the id/email specified.
//     if the user doesn't have an email, the one from the order is used
// 4 - if the order doesn't have an email, but the user does, we will use that one
//
func setOrderEmail(tx *gorm.DB, order *models.Order, claims *claims.JWTClaims, log logrus.FieldLogger) *HTTPError {
	if claims == nil {
		log.Debug("No claims provided, proceeding as an anon request")
	} else {
		if claims.Subject == "" {
			return badRequestError("Token had an invalid ID: %s", claims.Subject)
		}

		log = log.WithField("user_id", claims.Subject)
		order.UserID = claims.Subject

		user := new(models.User)
		result := tx.First(user, "id = ?", claims.Subject)
		if result.RecordNotFound() {
			log.Debugf("Didn't find a user for id %s ~ going to create one", claims.Subject)
			user.ID = claims.Subject
			user.Email = claims.Email
			tx.Create(user)
		} else if result.Error != nil {
			return internalServerError("Token had an invalid ID").WithInternalError(result.Error)
		}

		if order.Email == "" {
			order.Email = user.Email
		}
	}

	if order.Email == "" {
		return badRequestError("Either the order parameters or the user must provide an email")
	}
	return nil
}

func (a *API) createLineItems(ctx context.Context, tx *gorm.DB, order *models.Order, items []*orderLineItem, log logrus.FieldLogger) *HTTPError {
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
		go func(item *models.LineItem, orderItem *orderLineItem) {
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
		return internalServerError("Error processing line item").WithInternalError(sharedErr.err)
	}

	for _, item := range order.LineItems {
		order.SubTotal = order.SubTotal + (item.Price+item.AddonPrice)*item.Quantity
		if err := tx.Save(&item).Error; err != nil {
			return internalServerError("Error creating line item").WithInternalError(err)
		}
	}

	for _, download := range order.Downloads {
		if err := tx.Create(&download).Error; err != nil {
			return internalServerError("Error creating download item").WithInternalError(err)
		}
	}

	settings, err := a.loadSettings(ctx)
	if err != nil {
		return internalServerError(err.Error()).WithInternalError(err)
	}

	order.CalculateTotal(settings, gcontext.GetClaimsAsMap(ctx), log)
	return nil
}

func (a *API) loadSettings(ctx context.Context) (*calculator.Settings, error) {
	config := gcontext.GetConfig(ctx)

	settings := &calculator.Settings{}
	resp, err := a.httpClient.Get(config.SettingsURL())
	if err != nil {
		return nil, fmt.Errorf("Error loading site settings: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
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
			return nil, badRequestError("Bad %v id: %v", name, id).WithInternalError(result.Error)
		}

		if order.UserID != loadedAddress.UserID {
			return nil, badRequestError("Can't update the order to an %v that doesn't belong to the user", name)
		}
		return loadedAddress, nil
	}

	address.UserID = order.UserID
	// it is a new address we're making
	if err := address.Validate(); err != nil {
		return nil, badRequestError("Failed to validate %v: %v", name, err.Error())
	}

	// is a valid id that doesn't already belong to a user
	address.ID = uuid.NewRandom().String()
	tx.Create(address)
	return address, nil
}

func (a *API) processLineItem(ctx context.Context, order *models.Order, item *models.LineItem, orderItem *orderLineItem) error {
	config := gcontext.GetConfig(ctx)
	jwtClaims := gcontext.GetClaimsAsMap(ctx)
	resp, err := a.httpClient.Get(config.SiteURL + item.Path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return err
	}

	metaTag := doc.Find(".gocommerce-product")
	if metaTag.Length() == 0 {
		return fmt.Errorf("No script tag with class gocommerce-product tag found for '%v'", item.Title)
	}
	metaProducts := []*models.LineItemMetadata{}
	var parsingErr error
	metaTag.EachWithBreak(func(_ int, tag *goquery.Selection) bool {
		meta := &models.LineItemMetadata{}
		parsingErr = json.Unmarshal([]byte(tag.Text()), meta)
		if parsingErr != nil {
			return false
		}
		metaProducts = append(metaProducts, meta)
		return true
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
				item.AddonItems = append(item.AddonItems, &models.AddonItem{
					Sku: addon.Sku,
				})
			}

			return item.Process(jwtClaims, order, meta)
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
