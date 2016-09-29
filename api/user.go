package api

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/Sirupsen/logrus"
	"github.com/guregu/kami"
	"github.com/jinzhu/gorm"
	"github.com/pborman/uuid"

	"github.com/netlify/netlify-commerce/models"
)

// UserList will return all of the users. It requires admin access.
// It supports the filters:
// since     iso8601 date
// before		 iso8601 date
// email     email
// user_id   id
// limit     # of records to return (max)
func (a *API) UserList(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	_, _, httpErr := checkPermissions(ctx, true)
	if httpErr != nil {
		sendJSON(w, httpErr.Code, httpErr)
		return
	}

	log := getLogger(ctx)

	query, err := parseUserQueryParams(a.db, r.URL.Query())
	if err != nil {
		log.WithError(err).Info("Bad query parameters in request")
		badRequestError(w, "Bad parameters in query: "+err.Error())
		return
	}

	log.Debug("Parsed url params")

	var users []models.User
	results := query.Find(&users)
	if results.Error != nil {
		log.WithError(results.Error).Warn("Error qhile querying the database")
		internalServerError(w, "Failed to execute request")
		return
	}

	numUsers := len(users)
	log.WithField("user_count", numUsers).Debugf("Successfully retrieved %d users", numUsers)
	sendJSON(w, 200, users)
}

// UserView will return the user specified.
// If you're an admin you can request a user that is not your self
func (a *API) UserView(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID, _, httpErr := checkPermissions(ctx, false)
	if httpErr != nil {
		sendJSON(w, httpErr.Code, httpErr)
		return
	}
	log := getLogger(ctx)

	user := &models.User{
		ID: userID,
	}
	rsp := a.db.First(user)
	if rsp.RecordNotFound() {
		notFoundError(w, "Couldn't find a record for "+userID)
		return
	}

	if rsp.Error != nil {
		log.WithError(rsp.Error).Warnf("Failed to query DB: %v", rsp.Error)
		internalServerError(w, "Problem searching for user "+userID)
	}

	if user.DeletedAt != nil {
		notFoundError(w, "Couldn't find a record for "+userID)
	}

	sendJSON(w, 200, user)
}

// AddressList will return the addresses for a given user
func (a *API) AddressList(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID, _, httpErr := checkPermissions(ctx, false)
	if httpErr != nil {
		sendJSON(w, httpErr.Code, httpErr)
		return
	}

	log := getLogger(ctx)

	if getUser(a.db, userID) == nil {
		log.WithError(notFoundError(w, "couldn't find a record for user: "+userID)).Warn("requested non-existent user")
		return
	}

	addrs := []models.Address{}
	results := a.db.Where("user_id = ?", userID).Find(&addrs)
	if results.Error != nil {
		log.WithError(results.Error).Warn("failed to query for userID: " + userID)
		internalServerError(w, "problem while querying for userID: "+userID)
		return
	}

	sendJSON(w, 200, &addrs)
}

// AddressView will return a particular address for a given user
func (a *API) AddressView(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID, addrID, httpErr := checkPermissions(ctx, false)
	if httpErr != nil {
		sendJSON(w, httpErr.Code, httpErr)
		return
	}

	log := getLogger(ctx)

	if getUser(a.db, userID) == nil {
		log.WithError(notFoundError(w, "couldn't find a record for user: "+userID)).Warn("requested non-existent user")
		return
	}

	addr := &models.Address{
		ID:     addrID,
		UserID: userID,
	}
	results := a.db.First(addr)
	if results.Error != nil {
		log.WithError(results.Error).Warn("failed to query for userID: " + userID)
		internalServerError(w, "problem while querying for userID: "+userID)
		return
	}

	sendJSON(w, 200, &addr)
}

// UserDelete will soft delete the user. It requires admin access
// return errors or 200 and no body
func (a *API) UserDelete(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID, _, httpErr := checkPermissions(ctx, true)
	if httpErr != nil {
		sendJSON(w, httpErr.Code, httpErr)
		return
	}
	log := getLogger(ctx)
	log.Debugf("Starting to delete user %s", userID)

	user := getUser(a.db, userID)
	if user == nil {
		log.Info("attempted to delete non-existent user")
		return // not an error ~ just an action
	}

	// do a cascading delete
	tx := a.db.Begin()

	results := tx.Delete(user)
	if results.Error != nil {
		tx.Rollback()
		log.WithError(results.Error).Warn("Failed to find associated orders")
		internalServerError(w, "Failed to delete user")
		return
	}
	log.Debug("Deleted user")

	orders := []models.Order{}
	results = tx.Where("user_id = ?", userID).Find(&orders)
	if results.Error != nil {
		tx.Rollback()
		log.WithError(results.Error).Warn("Failed to find associated orders")
		internalServerError(w, "Failed to delete user")
		return
	}

	log.Debugf("Starting to collect info about %d orders", len(orders))
	orderIDs := []string{}
	for _, o := range orders {
		orderIDs = append(orderIDs, o.ID)
	}

	log.Debugf("Deleting line items")
	results = tx.Where("order_id in (?)", orderIDs).Delete(&models.LineItem{})
	if results.Error != nil {
		tx.Rollback()
		log.WithError(results.Error).
			WithField("order_ids", orderIDs).
			Warnf("Failed to delete line items associated with orders: %v", orderIDs)
		internalServerError(w, "Failed to delete user")
		return
	}
	log.Debugf("Deleted %d items", results.RowsAffected)

	if err := tryDelete(tx, w, log, userID, &models.Order{}); err != nil {
		return
	}
	if err := tryDelete(tx, w, log, userID, &models.Transaction{}); err != nil {
		return
	}
	if err := tryDelete(tx, w, log, userID, &models.OrderNote{}); err != nil {
		return
	}
	if err := tryDelete(tx, w, log, userID, &models.Address{}); err != nil {
		return
	}

	tx.Commit()
	log.Infof("Deleted user")
}

// AddressDelete will soft delete the address associated with that user. It requires admin access
// return errors or 200 and no body
func (a *API) AddressDelete(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID, addrID, httpErr := checkPermissions(ctx, true)
	if httpErr != nil {
		sendJSON(w, httpErr.Code, httpErr)
		return
	}
	log := getLogger(ctx).WithField("addr_id", addrID)

	if getUser(a.db, userID) == nil {
		log.Warn("requested non-existent user - not an error b/c it is a delete")
		return
	}

	rsp := a.db.Delete(&models.Address{ID: addrID})
	if rsp.RecordNotFound() {
		log.Warn("Attempted to delete an address that doesn't exist")
		return
	} else if rsp.Error != nil {
		log.WithError(rsp.Error).Warn("Error while deleting address")
		internalServerError(w, "error while deleting address")
		return
	}

	log.Info("deleted address")
}

// CreateNewAddress will create an address associated with that user
func (a *API) CreateNewAddress(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID, _, httpErr := checkPermissions(ctx, true)
	if httpErr != nil {
		sendJSON(w, httpErr.Code, httpErr)
		return
	}
	log := getLogger(ctx)

	if getUser(a.db, userID) == nil {
		log.WithError(notFoundError(w, "Couldn't find user "+userID)).Warn("Requested to add an address to a missing user")
		return
	}

	addrReq := new(models.AddressRequest)
	err := json.NewDecoder(r.Body).Decode(addrReq)
	if err != nil {
		log.WithError(err).Info("Failed to parse json")
		badRequestError(w, "Failed to parse json body")
		return
	}

	if err := addrReq.Validate(); err != nil {
		log.WithError(err).Infof("requested address is not valid")
		badRequestError(w, "requested address is missing a required field: "+err.Error())
		return
	}

	addr := models.Address{
		AddressRequest: *addrReq,
		ID:             uuid.NewRandom().String(),
		UserID:         userID,
	}
	rsp := a.db.Create(&addr)
	if rsp.Error != nil {
		log.WithError(rsp.Error).Warnf("Failed to save address %v", addr)
		internalServerError(w, "failed to save address")
		return
	}

	sendJSON(w, 200, &struct{ ID string }{ID: addr.ID})
}

// -------------------------------------------------------------------------------------------------------------------
// Helper methods
// -------------------------------------------------------------------------------------------------------------------
func checkPermissions(ctx context.Context, adminOnly bool) (string, string, *HTTPError) {
	log := getLogger(ctx)
	userID := kami.Param(ctx, "user_id")
	addrID := kami.Param(ctx, "addr_id")

	claims := getClaims(ctx)
	if claims == nil {
		err := httpError(401, "No claims provided")
		log.WithError(err).Warn("Illegal access attempt")
		return "", "", err
	}

	isAdmin := isAdmin(ctx)
	if isAdmin {
		ctx = withLogger(ctx, log.WithField("admin_id", claims.ID))
	}

	if claims.ID != userID && !isAdmin {
		err := httpError(401, "Can't access a different user unless you're an admin")
		log.WithError(err).Warn("Illegal access attempt")
		return "", "", err
	}

	if adminOnly && !isAdmin {
		err := httpError(401, "Admin permissions required")
		log.WithError(err).Warn("Illegal access attempt")
		return "", "", err
	}

	return userID, addrID, nil
}

func getUser(db *gorm.DB, userID string) *models.User {
	user := &models.User{ID: userID}
	results := db.Find(user)
	if results.RecordNotFound() {
		return nil
	}

	return user
}

func tryDelete(tx *gorm.DB, w http.ResponseWriter, log *logrus.Entry, userID string, face interface{}) error {
	typeName := reflect.TypeOf(face).String()

	log.WithField("type", typeName).Debugf("Starting to delete %s", typeName)
	results := tx.Where("user_id = ?", userID).Delete(face)
	if results.Error != nil {
		tx.Rollback()
		log.WithError(results.Error).Warnf("Failed to delete %s", typeName)
		internalServerError(w, "Failed to delete user")
	}

	log.WithField("affected_rows", results.RowsAffected).Debugf("Deleted %d rows", results.RowsAffected)
	return results.Error
}
