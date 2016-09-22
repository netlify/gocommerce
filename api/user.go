package api

import (
	"net/http"

	"github.com/guregu/kami"
	"github.com/jinzhu/gorm"
	"golang.org/x/net/context"

	"github.com/netlify/gocommerce/models"
)

// ENDPOINTS
//
// GET
// /users/
// /users/:id
// /users/:id/addresses
// /users/:id/addresses/:id
//
// DELETE
// /users/:id
// /users/:id/addresses/:id
//
// POST
// /users								(create user)
// /users/:id						(modify email)
// /users/:id/addresses (create an address)
//

// GetAllUsers will return all of the users. It requires admin access.
// It supports the filters:
// since     iso8601 date
// before		 iso8601 date
// email     email
// user_id   id
// limit     # of records to return (max)
func (a *API) GetAllUsers(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	_, _, httpError := checkPermissions(ctx, true)
	if httpError != nil {
		sendError(w, httpError)
		return
	}

	log := Logger(ctx)

	query, err := parseUserQueryParams(a.db, r.URL.Query())
	if err != nil {
		log.WithError(err).Info("Bad query parameters in request")
		BadRequestError(w, "Bad parameters in query: "+err.Error())
		return
	}

	log.Debug("Parsed url params")

	var users []models.User
	results := query.Find(&users)
	if results.Error != nil {
		log.WithError(results.Error).Warn("Error qhile querying the database")
		InternalServerError(w, "Failed to execute request")
		return
	}

	numUsers := len(users)
	log.WithField("user_count", numUsers).Debugf("Successfully retrieved %d users", numUsers)
	sendJSON(w, 200, users)
}

// GetSingleUser will return the user specified.
// If you're an admin you can request a user that is not your self
func (a *API) GetSingleUser(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID, _, httpErr := checkPermissions(ctx, false)
	if httpErr != nil {
		sendError(w, httpErr)
		return
	}
	log := Logger(ctx)

	user := &models.User{
		ID: userID,
	}
	rsp := a.db.First(user)
	if rsp.RecordNotFound() {
		NotFoundError(w, "Couldn't find a record for "+userID)
		return
	}

	if rsp.Error != nil {
		log.WithError(rsp.Error).Warnf("Failed to query DB: %v", rsp.Error)
		InternalServerError(w, "Problem searching for user "+userID)
	}

	if user.DeletedAt != nil {
		NotFoundError(w, "Couldn't find a record for "+userID)
	}

	sendJSON(w, 200, user)
}

// GetAllAddresses will return the addresses for a given user
func (a *API) GetAllAddresses(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID, _, httpErr := checkPermissions(ctx, false)
	if httpErr != nil {
		sendError(w, httpErr)
		return
	}

	log := Logger(ctx)

	if !userExists(a.db, userID) {
		log.WithError(NotFoundError(w, "couldn't find a record for user: "+userID)).Warn("requested non-existent user")
		return
	}

	addrs := []models.Address{}
	results := a.db.Where("user_id = ?", userID).Find(&addrs)
	if results.Error != nil {
		log.WithError(results.Error).Warn("failed to query for userID: " + userID)
		InternalServerError(w, "problem while querying for userID: "+userID)
		return
	}

	sendJSON(w, 200, &addrs)
}

// GetSingleAddress will return a particular address for a given user
func (a *API) GetSingleAddress(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID, addrID, httpErr := checkPermissions(ctx, false)
	if httpErr != nil {
		sendError(w, httpErr)
		return
	}

	log := Logger(ctx)

	if !userExists(a.db, userID) {
		log.WithError(NotFoundError(w, "couldn't find a record for user: "+userID)).Warn("requested non-existent user")
		return
	}

	addr := &models.Address{
		ID:     addrID,
		UserID: userID,
	}
	results := a.db.First(addr)
	if results.Error != nil {
		log.WithError(results.Error).Warn("failed to query for userID: " + userID)
		InternalServerError(w, "problem while querying for userID: "+userID)
		return
	}

	sendJSON(w, 200, &addr)
}

// DeleteSingleUser will soft delete the user. It requires admin access
// return errors or 200 and no body
func (a *API) DeleteSingleUser(ctx context.Context, w http.ResponseWriter, r *http.Request) {
}

// DeleteSingleAddress will soft delete the address associated with that user. It requires admin access
// return errors or 200 and no body
func (a *API) DeleteSingleAddress(ctx context.Context, w http.ResponseWriter, r *http.Request) {
}

// CreateNewUser will create a user with the info specified.
func (a *API) CreateNewUser(ctx context.Context, w http.ResponseWriter, r *http.Request) {
}

// CreateNewAddress will create an address associated with that user
func (a *API) CreateNewAddress(ctx context.Context, w http.ResponseWriter, r *http.Request) {
}

// UpdateUserEmail will update the user's email
func (a *API) UpdateUserEmail(ctx context.Context, w http.ResponseWriter, r *http.Request) {
}

// -------------------------------------------------------------------------------------------------------------------
// Helper methods
// -------------------------------------------------------------------------------------------------------------------
func checkPermissions(ctx context.Context, adminOnly bool) (string, string, *HTTPError) {
	log := Logger(ctx)
	userID := kami.Param(ctx, "user_id")
	addrID := kami.Param(ctx, "addr_id")

	claims := Claims(ctx)
	if claims == nil {
		err := httpError(401, "No claims provided")
		log.WithError(err).Warn("Illegal access attempt")
		return "", "", err
	}

	isAdmin := IsAdmin(ctx)
	if isAdmin {
		ctx = WithLogger(ctx, log.WithField("admin_id", claims.ID))
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

func userExists(db *gorm.DB, userID string) bool {
	results := db.Find(&models.User{ID: userID})
	return !results.RecordNotFound()
}
