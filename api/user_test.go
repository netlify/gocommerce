package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netlify/gocommerce/models"
)

func TestUsersQueryForAllUsersAsStranger(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(nil, config, false)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://example.com/users", nil).WithContext(ctx)

	token := testToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)
	validateError(t, http.StatusUnauthorized, recorder)
}

func TestUsersQueryForAllUsersWithParams(t *testing.T) {
	db, globalConfig, config := db(t)
	toDie := models.User{
		ID:    "villian",
		Email: "twoface@dc.com",
	}
	rsp := db.Create(&toDie)
	if rsp.Error != nil {
		assert.FailNow(t, "failed b/c of db error: "+rsp.Error.Error())
	}
	defer db.Unscoped().Delete(&toDie)

	ctx := testContext(nil, config, true)

	req := httptest.NewRequest("GET", "http://example.com/users?email=dc.com", nil).WithContext(ctx)
	recorder := httptest.NewRecorder()

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)

	users := []models.User{}
	extractPayload(t, http.StatusOK, recorder, &users)
	assert.Equal(t, 1, len(users))
	assert.Equal(t, "villian", users[0].ID)
}

func TestUsersQueryForAllUsers(t *testing.T) {
	db, globalConfig, config := db(t)
	toDie := models.User{
		ID:    "villian",
		Email: "twoface@dc.com",
	}
	db.Create(&toDie)
	defer db.Unscoped().Delete(&toDie)

	recorder := httptest.NewRecorder()
	ctx := testContext(nil, config, true)
	req := httptest.NewRequest("GET", "https://example.com/users", nil).WithContext(ctx)

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)

	users := []models.User{}
	extractPayload(t, http.StatusOK, recorder, &users)
	for _, u := range users {
		switch u.ID {
		case toDie.ID:
			assert.Equal(t, "twoface@dc.com", u.Email)
		case testUser.ID:
			assert.Equal(t, testUser.Email, u.Email)
		default:
			assert.Fail(t, "unexpected user %v\n", u)
		}
	}
}

//func TestUsersQueryForDeletedUser(t *testing.T) {
//	toDie := models.User{
//		ID:    "def-should-not-exist",
//		Email: "twoface@dc.com",
//	}
//	db.Create(&toDie)
//	db.Delete(&toDie) // soft delete
//	defer db.Unscoped().Delete(&toDie)
//
//	recorder := httptest.NewRecorder()
//	req := httptest.NewRequest("GET", urlWithUserID, nil)
//
//	globalConfig := testConfig()
//	ctx := testContext(testToken(toDie.ID, toDie.Email, nil), globalConfig)
//	chi.RouteContext(ctx).URLParams.Add("user_id", toDie.ID)
//
//	api := NewAPI(globalConfig, db, nil)
//	api.UserView(ctx, recorder, req)
//	validateError(t, http.StatusNotFound, recorder)
//}

func TestUsersQueryForUserAsUser(t *testing.T) {
	db, globalConfig, config := db(t)
	recorder := httptest.NewRecorder()

	ctx := testContext(nil, config, false)
	req := httptest.NewRequest("GET", "https://example.com/users/"+testUser.ID, nil).WithContext(ctx)

	token := testToken(testUser.ID, testUser.Email)
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)

	user := new(models.User)
	extractPayload(t, http.StatusOK, recorder, user)

	validateUser(t, &testUser, user)
}

func TestUsersQueryForUserAsStranger(t *testing.T) {
	db, globalConfig, config := db(t)
	recorder := httptest.NewRecorder()
	ctx := testContext(nil, config, false)

	req := httptest.NewRequest("GET", "https://example.com/users/"+testUser.ID, nil).WithContext(ctx)

	token := testToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)
	validateError(t, http.StatusUnauthorized, recorder)
}

func TestUsersQueryForUserAsAdmin(t *testing.T) {
	db, globalConfig, config := db(t)
	recorder := httptest.NewRecorder()
	ctx := testContext(nil, config, true)

	req := httptest.NewRequest("GET", "https://example.com/users/"+testUser.ID, nil).WithContext(ctx)

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)

	user := new(models.User)
	extractPayload(t, http.StatusOK, recorder, user)
	validateUser(t, &testUser, user)
}

func TestUsersQueryForAllAddressesAsAdmin(t *testing.T) {
	db, globalConfig, config := db(t)
	second := getTestAddress()
	second.UserID = testUser.ID
	assert.Nil(t, second.Validate())
	db.Create(&second)
	defer db.Unscoped().Delete(&second)

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)

	addrs := queryForAddresses(t, NewAPI(globalConfig, config, db), testUser.ID, tokenStr)
	assert.Equal(t, 2, len(addrs))
	for _, a := range addrs {
		assert.Nil(t, a.Validate())
		switch a.ID {
		case second.ID:
			validateAddress(t, *second, a)
		case testAddress.ID:
			validateAddress(t, testAddress, a)
		default:
			assert.Fail(t, fmt.Sprintf("Unexpected address: %+v", a))
		}
	}
}

func TestUsersQueryForAllAddressesAsUser(t *testing.T) {
	db, globalConfig, config := db(t)

	token := testToken(testUser.ID, "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)

	addrs := queryForAddresses(t, NewAPI(globalConfig, config, db), testUser.ID, tokenStr)
	assert.Equal(t, 1, len(addrs))
	validateAddress(t, testAddress, addrs[0])
}

func TestUsersQueryForAllAddressesAsStranger(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(nil, config, false)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://example.com/users/"+testUser.ID+"/addresses", nil).WithContext(ctx)

	token := testToken("stranger-danger", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)
	validateError(t, http.StatusUnauthorized, recorder)
}

func TestUsersQueryForAllAddressesNoAddresses(t *testing.T) {
	db, globalConfig, config := db(t)
	u := models.User{
		ID:    "temporary",
		Email: "junk@junk.com",
	}
	db.Create(u)
	defer db.Unscoped().Delete(u)

	token := testToken(u.ID, "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)

	addrs := queryForAddresses(t, NewAPI(globalConfig, config, db), u.ID, tokenStr)
	assert.Equal(t, 0, len(addrs))
}

func TestUsersQueryForAllAddressesMissingUser(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(nil, config, false)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://example.com/users/dne/addresses", nil).WithContext(ctx)

	token := testToken("dne", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)
	validateError(t, http.StatusNotFound, recorder)
}

func TestUsersQueryForSingleAddressAsUser(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(nil, config, false)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://example.com/users/"+testUser.ID+"/addresses/"+testAddress.ID, nil).WithContext(ctx)

	token := testToken(testUser.ID, "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)

	addr := new(models.Address)
	extractPayload(t, http.StatusOK, recorder, addr)
	validateAddress(t, testAddress, *addr)
}

func TestUsersDeleteNonExistentUser(t *testing.T) {
	db, globalConfig, config := db(t)
	ctx := testContext(nil, config, true)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "https://example.com/users/dne", nil).WithContext(ctx)

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "", recorder.Body.String())
}

func TestUsersDeleteSingleUser(t *testing.T) {
	db, globalConfig, config := db(t)
	dyingUser := models.User{ID: "going-to-die", Email: "nobody@nowhere.com"}
	dyingAddr := getTestAddress()
	dyingAddr.UserID = dyingUser.ID
	dyingOrder := models.NewOrder("session2", dyingUser.Email, "usd")
	dyingOrder.UserID = dyingUser.ID
	dyingTransaction := models.NewTransaction(dyingOrder)
	dyingTransaction.UserID = dyingUser.ID
	dyingLineItem := models.LineItem{
		ID:          123,
		OrderID:     dyingOrder.ID,
		Title:       "coffin",
		Sku:         "123-cough-cough-123",
		Type:        "home",
		Description: "nappytimeplace",
		Price:       100,
		Quantity:    1,
		Path:        "/right/to/the/grave",
	}
	items := []interface{}{&dyingUser, &dyingAddr, dyingOrder, &dyingLineItem, &dyingTransaction}
	for _, i := range items {
		db.Create(i)
	}
	defer func() {
		for _, i := range items {
			db.Unscoped().Delete(i)
		}
	}()

	ctx := testContext(nil, config, true)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "https://example.com/users/"+dyingUser.ID, nil).WithContext(ctx)

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "", recorder.Body.String())

	// now load it back and it should be soft deleted
	//found := &models.User{ID: dyingUser.ID}
	assert.False(t, db.Unscoped().First(&dyingUser).RecordNotFound())
	assert.NotNil(t, dyingUser.DeletedAt, "user wasn't deleted")
	assert.False(t, db.Unscoped().First(&dyingAddr).RecordNotFound())
	assert.NotNil(t, dyingAddr.DeletedAt, "addr wasn't deleted")
	assert.False(t, db.Unscoped().First(dyingOrder).RecordNotFound())
	assert.NotNil(t, dyingOrder.DeletedAt, "order wasn't deleted")
	assert.False(t, db.Unscoped().First(&dyingTransaction).RecordNotFound())
	assert.NotNil(t, dyingTransaction.DeletedAt, "transaction wasn't deleted")
	assert.False(t, db.Unscoped().First(&dyingLineItem).RecordNotFound())
	assert.NotNil(t, dyingLineItem.DeletedAt, "line item wasn't deleted")
}

func TestDeleteUserAddress(t *testing.T) {
	db, globalConfig, config := db(t)
	addr := getTestAddress()
	addr.UserID = testUser.ID
	db.Create(addr)

	ctx := testContext(nil, config, true)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "https://example.com/users/"+testUser.ID+"/addresses/"+addr.ID, nil).WithContext(ctx)

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "", recorder.Body.String())

	assert.False(t, db.Unscoped().First(&addr).RecordNotFound())
	assert.NotNil(t, addr.DeletedAt)
}

func TestCreateAnAddress(t *testing.T) {
	db, globalConfig, config := db(t)
	addr := getTestAddress()
	b, err := json.Marshal(&addr.AddressRequest)
	assert.Nil(t, err)

	ctx := testContext(nil, config, true)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "https://example.com/users/"+testUser.ID+"/addresses", bytes.NewBuffer(b)).WithContext(ctx)

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)

	results := struct {
		ID string
	}{}
	err = json.Unmarshal(recorder.Body.Bytes(), &results)
	assert.Nil(t, err)

	// now pull off the address from the DB
	dbAddr := &models.Address{ID: results.ID, UserID: testUser.ID}
	rsp := db.First(dbAddr)
	assert.False(t, rsp.RecordNotFound())
}

func TestCreateInvalidAddress(t *testing.T) {
	db, globalConfig, config := db(t)
	addr := getTestAddress()
	addr.LastName = "" // required field

	b, err := json.Marshal(&addr.AddressRequest)
	assert.Nil(t, err)

	ctx := testContext(nil, config, true)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "https://example.com/users/"+testUser.ID+"/addresses", bytes.NewBuffer(b)).WithContext(ctx)

	token := testAdminToken("magical-unicorn", "")
	tokenStr, err := token.SignedString([]byte(config.JWT.Secret))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	NewAPI(globalConfig, config, db).handler.ServeHTTP(recorder, req)
	validateError(t, http.StatusBadRequest, recorder)
}

// ------------------------------------------------------------------------------------------------

func queryForAddresses(t *testing.T, api *API, id string, tokenStr string) []models.Address {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://example.com/users/"+id+"/addresses", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenStr))

	api.handler.ServeHTTP(recorder, req)

	addrs := []models.Address{}
	extractPayload(t, http.StatusOK, recorder, &addrs)
	return addrs
}
