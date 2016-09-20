package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/guregu/kami"
	"github.com/stretchr/testify/assert"

	"github.com/netlify/gocommerce/models"
	tu "github.com/netlify/gocommerce/testutils"
)

func TestUsersQueryForAllUsers(t *testing.T) {
}

func TestUsersQueryForDeletedUser(t *testing.T) {
	toDie := models.User{
		ID:    "def-should-not-exist",
		Email: "twoface@dc.com",
	}
	db.Create(&toDie)
	db.Delete(&toDie) // soft delete
	defer db.Unscoped().Delete(&toDie)

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", urlWithUserID, nil)

	ctx := testContext(token(toDie.ID, toDie.Email, nil))
	ctx = kami.SetParam(ctx, "user_id", toDie.ID)

	api := NewAPI(config, db, nil)
	api.GetSingleUser(ctx, recorder, req)
	validateError(t, 404, recorder.Body)
}

func TestUsersQueryForUserAsUser(t *testing.T) {
	config := testConfig()
	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", urlWithUserID, nil)

	ctx := testContext(token(tu.TestUser.ID, tu.TestUser.Email, nil), config)
	ctx = kami.SetParam(ctx, "user_id", tu.TestUser.ID)

	api := NewAPI(config, db, nil)
	api.GetSingleUser(ctx, recorder, req)
	user := extractUser(t, 200, recorder)

	validateUser(t, &tu.TestUser, user)
}

func TestUsersQueryForUserAsStranger(t *testing.T) {
	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", urlWithUserID, nil)

	config := testConfig()
	ctx := testContext(token("magical-unicorn", "", nil), config)
	ctx = kami.SetParam(ctx, "user_id", tu.TestUser.ID)

	api := NewAPI(config, db, nil)
	api.GetSingleUser(ctx, recorder, req)
	validateError(t, 401, recorder.Body)
}

func TestUsersQueryForUserAsAdmin(t *testing.T) {
	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", urlWithUserID, nil)

	config := testConfig()
	config.JWT.AdminGroupName = "admin"
	ctx := testContext(token("magical-unicorn", "", &[]string{"admin"}), config)
	ctx = kami.SetParam(ctx, "user_id", tu.TestUser.ID)

	api := NewAPI(config, db, nil)
	api.GetSingleUser(ctx, recorder, req)

	user := extractUser(t, 200, recorder)
	validateUser(t, &tu.TestUser, user)
}

// --------------------------------------------------------------------------------------------------------------------
//
// --------------------------------------------------------------------------------------------------------------------
func validateUser(t *testing.T, expected *models.User, actual *models.User) {
	assert := assert.New(t)
	assert.Equal(expected.ID, actual.ID)
	assert.Equal(expected.Email, actual.Email)
}

func extractUser(t *testing.T, code int, recorder *httptest.ResponseRecorder) *models.User {
	user := new(models.User)
	err := json.NewDecoder(recorder.Body).Decode(user)
	assert.NoError(t, err)
	assert.Equal(t, code, recorder.Code)
	return user
}
