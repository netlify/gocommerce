package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"reflect"
	"time"

	"github.com/go-chi/chi"
	"github.com/jinzhu/gorm"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"

	gcontext "github.com/netlify/gocommerce/context"
	"github.com/netlify/gocommerce/models"
)

func (a *API) withUserID(w http.ResponseWriter, r *http.Request) (context.Context, error) {
	userID := chi.URLParam(r, "user_id")
	logEntrySetField(r, "user_id", userID)

	ctx := gcontext.WithUserID(r.Context(), userID)
	return ctx, nil
}

// UserList will return all of the users. It requires admin access.
// It supports the filters:
// since     iso8601 date
// before		 iso8601 date
// email     email
// user_id   id
// limit     # of records to return (max)
func (a *API) UserList(w http.ResponseWriter, r *http.Request) error {
	log := getLogEntry(r)

	query, err := parseUserQueryParams(a.db, r.URL.Query())
	if err != nil {
		return badRequestError("Bad parameters in query: %v", err)
	}

	log.Debug("Parsed url params")

	var users []models.User
	orderTable := a.db.NewScope(models.Order{}).QuotedTableName()
	userTable := a.db.NewScope(models.User{}).QuotedTableName()
	query = query.
		Joins("LEFT JOIN " + orderTable + " as orders ON " + userTable + ".id = orders.user_id").
		Select(userTable + ".id, " + userTable + ".email, " + userTable + ".created_at, " + userTable + ".updated_at, count(orders.id) as order_count").
		Group(userTable + ".id")

	offset, limit, err := paginate(w, r, query.Model(&models.User{}))
	if err != nil {
		if err == sql.ErrNoRows {
			return sendJSON(w, http.StatusOK, []string{})
		}
		return badRequestError("Bad Pagination Parameters: %v", err)
	}

	rows, err := query.Offset(offset).Limit(limit).Find(&users).Rows()
	if err != nil {
		return internalServerError("Failed to execute request").WithInternalError(err)
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		var id, email string
		var createdAt, updatedAt time.Time
		var orderCount int64
		err := rows.Scan(&id, &email, &createdAt, &updatedAt, &orderCount)
		if err != nil {
			return internalServerError("Failed to execute request").WithInternalError(err)
		}
		users[i].OrderCount = orderCount
		i++
	}

	numUsers := len(users)
	log.WithField("user_count", numUsers).Debugf("Successfully retrieved %d users", numUsers)
	return sendJSON(w, http.StatusOK, users)
}

// UserView will return the user specified.
// If you're an admin you can request a user that is not your self
func (a *API) UserView(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	userID := gcontext.GetUserID(ctx)

	user := &models.User{
		ID: userID,
	}
	rsp := a.db.First(user)
	if rsp.RecordNotFound() {
		return notFoundError("Couldn't find a record for " + userID)
	}

	if rsp.Error != nil {
		return internalServerError("Problem searching for user %s", userID).WithInternalError(rsp.Error)
	}

	if user.DeletedAt != nil {
		return notFoundError("Couldn't find a record for " + userID)
	}

	orders := []models.Order{}
	a.db.Where("user_id = ?", user.ID).Find(&orders).Count(&user.OrderCount)

	return sendJSON(w, http.StatusOK, user)
}

// AddressList will return the addresses for a given user
func (a *API) AddressList(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	userID := gcontext.GetUserID(ctx)

	if getUser(a.db, userID) == nil {
		return notFoundError("couldn't find a record for user: " + userID)
	}

	addrs := []models.Address{}
	results := a.db.Where("user_id = ?", userID).Find(&addrs)
	if results.Error != nil {
		return internalServerError("problem while querying for userID: %s", userID).WithInternalError(results.Error)
	}

	return sendJSON(w, http.StatusOK, &addrs)
}

// AddressView will return a particular address for a given user
func (a *API) AddressView(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	addrID := chi.URLParam(r, "addr_id")
	userID := gcontext.GetUserID(ctx)

	if getUser(a.db, userID) == nil {
		return notFoundError("couldn't find a record for user: " + userID)
	}

	addr := &models.Address{
		ID:     addrID,
		UserID: userID,
	}
	results := a.db.First(addr)
	if results.Error != nil {
		return internalServerError("problem while querying for userID: %s", userID).WithInternalError(results.Error)
	}

	return sendJSON(w, http.StatusOK, &addr)
}

// UserDelete will soft delete the user. It requires admin access
// return errors or 200 and no body
func (a *API) UserDelete(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	userID := gcontext.GetUserID(ctx)
	log := getLogEntry(r)
	log.Debugf("Starting to delete user %s", userID)

	user := getUser(a.db, userID)
	if user == nil {
		log.Info("attempted to delete non-existent user")
		return nil // not an error ~ just an action
	}

	// do a cascading delete
	tx := a.db.Begin()

	results := tx.Delete(user)
	if results.Error != nil {
		tx.Rollback()
		return internalServerError("Failed to delete user").WithInternalError(results.Error)
	}
	log.Debug("Deleted user")

	orders := []models.Order{}
	results = tx.Where("user_id = ?", userID).Find(&orders)
	if results.Error != nil {
		tx.Rollback()
		return internalServerError("Failed to delete user").WithInternalError(results.Error).WithInternalMessage("failed to find associated orders")
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
		return internalServerError("Failed to delete user").WithInternalError(results.Error).WithInternalMessage("Failed to delete line items associated with orders: %v", orderIDs)
	}
	log.Debugf("Deleted %d items", results.RowsAffected)

	if err := tryDelete(tx, w, log, userID, &models.Order{}); err != nil {
		return err
	}
	if err := tryDelete(tx, w, log, userID, &models.Transaction{}); err != nil {
		return err
	}
	if err := tryDelete(tx, w, log, userID, &models.OrderNote{}); err != nil {
		return err
	}
	if err := tryDelete(tx, w, log, userID, &models.Address{}); err != nil {
		return err
	}

	tx.Commit()
	log.Infof("Deleted user")
	return nil
}

// AddressDelete will soft delete the address associated with that user. It requires admin access
// return errors or 200 and no body
func (a *API) AddressDelete(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	addrID := chi.URLParam(r, "addr_id")
	userID := gcontext.GetUserID(ctx)
	log := getLogEntry(r).WithField("addr_id", addrID)

	if getUser(a.db, userID) == nil {
		log.Warn("requested non-existent user - not an error b/c it is a delete")
		return nil
	}

	rsp := a.db.Delete(&models.Address{ID: addrID})
	if rsp.RecordNotFound() {
		log.Warn("Attempted to delete an address that doesn't exist")
		return nil
	} else if rsp.Error != nil {
		return internalServerError("error while deleting address").WithInternalError(rsp.Error)
	}

	log.Info("deleted address")
	return nil
}

// CreateNewAddress will create an address associated with that user
func (a *API) CreateNewAddress(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	userID := gcontext.GetUserID(ctx)

	if getUser(a.db, userID) == nil {
		return notFoundError("Couldn't find user " + userID)
	}

	addrReq := new(models.AddressRequest)
	err := json.NewDecoder(r.Body).Decode(addrReq)
	if err != nil {
		return badRequestError("Failed to parse json body: %v", err)
	}

	if err := addrReq.Validate(); err != nil {
		return badRequestError("requested address is missing a required field: %v", err)
	}

	addr := models.Address{
		AddressRequest: *addrReq,
		ID:             uuid.NewRandom().String(),
		UserID:         userID,
	}
	rsp := a.db.Create(&addr)
	if rsp.Error != nil {
		return internalServerError("failed to save address").WithInternalError(rsp.Error)
	}

	return sendJSON(w, http.StatusOK, &struct{ ID string }{ID: addr.ID})
}

// -------------------------------------------------------------------------------------------------------------------
// Helper methods
// -------------------------------------------------------------------------------------------------------------------
func getUser(db *gorm.DB, userID string) *models.User {
	user := &models.User{ID: userID}
	results := db.Find(user)
	if results.RecordNotFound() {
		return nil
	}

	return user
}

func tryDelete(tx *gorm.DB, w http.ResponseWriter, log logrus.FieldLogger, userID string, face interface{}) error {
	typeName := reflect.TypeOf(face).String()

	log.WithField("type", typeName).Debugf("Starting to delete %s", typeName)
	results := tx.Where("user_id = ?", userID).Delete(face)
	if results.Error != nil {
		tx.Rollback()
		return internalServerError("Failed to delete user").WithInternalError(results.Error)
	}

	log.WithField("affected_rows", results.RowsAffected).Debugf("Deleted %d rows", results.RowsAffected)
	return results.Error
}
