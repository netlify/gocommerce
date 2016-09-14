package api

import (
	"net/http"

	"github.com/guregu/kami"
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
}

// GetSingleUser will return the user specified.
// If you're an admin you can request a user that is not your self
func (a *API) GetSingleUser(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	log := Logger(ctx)

	userID, _, httpErr := checkPermissions(ctx)
	if httpErr != nil {
		sendError(w, httpErr)
		return
	}

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
}

// GetSingleAddress will return a particular address for a given user
func (a *API) GetSingleAddress(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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
func checkPermissions(ctx context.Context) (string, string, *HTTPError) {
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

	return userID, addrID, nil
}
