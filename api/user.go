package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/jinzhu/gorm"
	"github.com/pborman/uuid"

	"github.com/netlify/gocommerce/claims"
	gcontext "github.com/netlify/gocommerce/context"
	"github.com/netlify/gocommerce/models"
)

func (a *API) withUser(w http.ResponseWriter, r *http.Request) (context.Context, error) {
	userID := chi.URLParam(r, "user_id")
	logEntrySetField(r, "user_id", userID)
	ctx := r.Context()

	if u, err := models.GetUser(a.db, userID); err != nil {
		return nil, internalServerError("problem while querying for userID: %s", userID).WithInternalError(err)
	} else if u != nil {
		ctx = gcontext.WithUser(ctx, u)
	}

	ctx = gcontext.WithUserID(ctx, userID)
	return ctx, nil
}

func findUserName(order *models.Order, claims *claims.JWTClaims) string {
	if rawName, ok := claims.UserMetaData["full_name"]; ok {
		if name, ok := rawName.(string); ok {
			return name
		}
	}
	if order.BillingAddress.Name != "" {
		return order.BillingAddress.Name
	}
	if order.ShippingAddress.Name != "" {
		return order.ShippingAddress.Name
	}
	return ""
}

// persistUserName will set a users name from a JWT or the order addresses
func persistUserName(tx *gorm.DB, order *models.Order, claims *claims.JWTClaims) *HTTPError {
	if claims == nil {
		return nil
	}

	if claims.Subject == "" {
		return badRequestError("Token had an invalid ID: %s", claims.Subject)
	}

	user := models.User{}
	if err := tx.Find(&user, "id = ?", claims.Subject).Error; err != nil {
		return internalServerError("User has not been created yet. This is unexpected behavior").
			WithInternalError(err)
	}

	name := findUserName(order, claims)
	if name != "" {
		user.Name = name
		if err := tx.Save(&user).Error; err != nil {
			return internalServerError("Unable to save user's name").WithInternalError(err)
		}
	}

	return nil
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

	orderTable := a.db.NewScope(models.Order{}).QuotedTableName()
	userTable := a.db.NewScope(models.User{}).QuotedTableName()
	query = query.
		Joins("LEFT JOIN " + orderTable + " ON " + userTable + ".id = " + orderTable + ".user_id").
		Group(userTable + ".id")

	instanceID := gcontext.GetInstanceID(r.Context())
	query = query.Where(userTable+".instance_id = ?", instanceID)

	offset, limit, err := paginate(w, r, query.Model(&models.User{}))
	if err != nil {
		if err == sql.ErrNoRows {
			return sendJSON(w, http.StatusOK, []string{})
		}
		return badRequestError("Bad Pagination Parameters: %v", err)
	}

	query = query.Select("" +
		"COUNT(" + orderTable + ".id) AS order_count, " +
		"MAX(" + orderTable + ".created_at) AS last_order_at, " +
		userTable + ".*")

	users := []models.User{}
	if err := query.Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		return internalServerError("Failed to execute request").WithInternalError(err)
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
	user := gcontext.GetUser(ctx)
	if user == nil {
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
	user := gcontext.GetUser(ctx)
	if user == nil {
		return notFoundError("Couldn't find a record for " + userID)
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
	user := gcontext.GetUser(ctx)
	if user == nil {
		return notFoundError("Couldn't find a record for " + userID)
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

	user := gcontext.GetUser(ctx)
	if user == nil {
		log.Info("attempted to delete non-existent user")
		return nil
	}

	rsp := a.db.Delete(user)
	if rsp.Error != nil {
		return internalServerError("error while deleting user").WithInternalError(rsp.Error)
	}

	log.Infof("Deleted user")
	return nil
}

func (a *API) UserBulkDelete(w http.ResponseWriter, r *http.Request) error {
	log := getLogEntry(r)

	query, err := parseUserBulkDeleteParams(a.db, r.URL.Query())
	if err != nil {
		return badRequestError("Bad parameters in query: %v", err)
	}

	users := []models.User{}
	if result := query.Find(&users); result.Error != nil {
		return internalServerError("error while deleting user").WithInternalError(result.Error)
	}

	tx := a.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, user := range users {
		if result := tx.Delete(&user); result.Error != nil {
			if result.RecordNotFound() {
				continue
			}
			tx.Rollback()
			return internalServerError("error while deleting user").WithInternalError(result.Error)
		}
	}

	log.Infof("Deleted users")
	return tx.Commit().Error
}

// AddressDelete will soft delete the address associated with that user. It requires admin access
// return errors or 200 and no body
func (a *API) AddressDelete(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	addrID := chi.URLParam(r, "addr_id")
	log := getLogEntry(r).WithField("addr_id", addrID)

	user := gcontext.GetUser(ctx)
	if user == nil {
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
	user := gcontext.GetUser(ctx)
	if user == nil {
		return notFoundError("Couldn't find a record for " + userID)
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
