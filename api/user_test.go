package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netlify/gocommerce/models"
)

func TestUsersList(t *testing.T) {
	t.Run("AsStranger", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodGet, "/users", nil, token)
		validateError(t, http.StatusUnauthorized, recorder)
	})
	t.Run("AsAdmin", func(t *testing.T) {
		test := NewRouteTest(t)
		toDie := models.User{
			ID:    "villian",
			Email: "twoface@dc.com",
		}
		rsp := test.DB.Create(&toDie)
		require.NoError(t, rsp.Error, "DB Error")
		defer test.DB.Unscoped().Delete(&toDie)

		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodGet, "/users", nil, token)

		users := []models.User{}
		extractPayload(t, http.StatusOK, recorder, &users)
		require.Len(t, users, 2)
		for _, u := range users {
			switch u.ID {
			case toDie.ID:
				assert.Equal(t, "twoface@dc.com", u.Email)
			case test.Data.testUser.ID:
				assert.Equal(t, test.Data.testUser.Email, u.Email)
			default:
				assert.Fail(t, "unexpected user %v\n", u)
			}
		}
	})
	t.Run("WithParams", func(t *testing.T) {
		test := NewRouteTest(t)
		toDie := models.User{
			ID:    "villian",
			Email: "twoface@dc.com",
		}
		rsp := test.DB.Create(&toDie)
		require.NoError(t, rsp.Error, "DB Error")
		defer test.DB.Unscoped().Delete(&toDie)

		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodGet, "/users?email=dc.com", nil, token)

		users := []models.User{}
		extractPayload(t, http.StatusOK, recorder, &users)
		require.Len(t, users, 1)
		assert.Equal(t, "villian", users[0].ID)
	})
}

func TestUsersView(t *testing.T) {
	t.Run("AsUser", func(t *testing.T) {
		test := NewRouteTest(t)
		url := "/users/" + test.Data.testUser.ID
		token := test.Data.testUserToken
		recorder := test.TestEndpoint(http.MethodGet, url, nil, token)

		user := new(models.User)
		extractPayload(t, http.StatusOK, recorder, user)
		validateUser(t, test.Data.testUser, user)
	})
	t.Run("AsStranger", func(t *testing.T) {
		test := NewRouteTest(t)
		url := "/users/" + test.Data.testUser.ID
		token := testToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodGet, url, nil, token)
		validateError(t, http.StatusUnauthorized, recorder)
	})
	t.Run("AsAdmin", func(t *testing.T) {
		test := NewRouteTest(t)
		url := "/users/" + test.Data.testUser.ID
		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodGet, url, nil, token)

		user := new(models.User)
		extractPayload(t, http.StatusOK, recorder, user)
		validateUser(t, test.Data.testUser, user)
	})
	t.Run("Deleted", func(t *testing.T) {
		test := NewRouteTest(t)
		toDie := models.User{
			ID:    "def-should-not-exist",
			Email: "twoface@dc.com",
		}
		test.DB.Create(&toDie)
		test.DB.Delete(&toDie) // soft delete
		defer test.DB.Unscoped().Delete(&toDie)

		token := testToken(toDie.ID, toDie.Email)
		recorder := test.TestEndpoint(http.MethodGet, "/users/"+toDie.ID, nil, token)
		validateError(t, http.StatusNotFound, recorder)
	})
}

func TestUserAddressesList(t *testing.T) {
	t.Run("AsAdmin", func(t *testing.T) {
		test := NewRouteTest(t)
		url := "/users/" + test.Data.testUser.ID + "/addresses"
		second := getTestAddress()
		second.UserID = test.Data.testUser.ID
		assert.Nil(t, second.Validate())
		test.DB.Create(&second)
		defer test.DB.Unscoped().Delete(&second)

		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodGet, url, nil, token)

		addrs := []models.Address{}
		extractPayload(t, http.StatusOK, recorder, &addrs)
		assert.Len(t, addrs, 2)
		for _, a := range addrs {
			assert.Nil(t, a.Validate())
			switch a.ID {
			case second.ID:
				validateAddress(t, *second, a)
			case test.Data.testAddress.ID:
				validateAddress(t, test.Data.testAddress, a)
			default:
				assert.Fail(t, fmt.Sprintf("Unexpected address: %+v", a))
			}
		}
	})
	t.Run("AsUser", func(t *testing.T) {
		test := NewRouteTest(t)
		url := "/users/" + test.Data.testUser.ID + "/addresses"
		token := testToken(test.Data.testUser.ID, "")
		recorder := test.TestEndpoint(http.MethodGet, url, nil, token)

		addrs := []models.Address{}
		extractPayload(t, http.StatusOK, recorder, &addrs)
		require.Len(t, addrs, 1)
		validateAddress(t, test.Data.testAddress, addrs[0])
	})
	t.Run("AsStranger", func(t *testing.T) {
		test := NewRouteTest(t)
		url := "/users/" + test.Data.testUser.ID + "/addresses"
		token := testToken("stranger-danger", "")
		recorder := test.TestEndpoint(http.MethodGet, url, nil, token)
		validateError(t, http.StatusUnauthorized, recorder)
	})
	t.Run("NoAddresses", func(t *testing.T) {
		test := NewRouteTest(t)
		u := models.User{
			ID:    "temporary",
			Email: "junk@junk.com",
		}
		test.DB.Create(u)
		defer test.DB.Unscoped().Delete(u)

		token := testToken(u.ID, "")
		recorder := test.TestEndpoint(http.MethodGet, "/users/"+u.ID+"/addresses", nil, token)
		addrs := []models.Address{}
		extractPayload(t, http.StatusOK, recorder, &addrs)
		assert.Len(t, addrs, 0)
	})
	t.Run("MissingUser", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testToken("dne", "")
		recorder := test.TestEndpoint(http.MethodGet, "/users/dne/addresses", nil, token)
		validateError(t, http.StatusNotFound, recorder)
	})
}

func TestUserAddressView(t *testing.T) {
	t.Run("AsUser", func(t *testing.T) {
		test := NewRouteTest(t)
		url := "/users/" + test.Data.testUser.ID + "/addresses/" + test.Data.testAddress.ID
		token := testToken(test.Data.testUser.ID, "")
		recorder := test.TestEndpoint(http.MethodGet, url, nil, token)

		addr := new(models.Address)
		extractPayload(t, http.StatusOK, recorder, addr)
		validateAddress(t, test.Data.testAddress, *addr)
	})
}

func TestUserDelete(t *testing.T) {
	t.Run("NonExistentUser", func(t *testing.T) {
		test := NewRouteTest(t)
		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodDelete, "/users/dne", nil, token)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "", recorder.Body.String())
	})
	t.Run("SingleUser", func(t *testing.T) {
		test := NewRouteTest(t)
		dyingUser := models.User{ID: "going-to-die", Email: "nobody@nowhere.com"}
		dyingAddr := getTestAddress()
		dyingAddr.UserID = dyingUser.ID
		dyingOrder := models.NewOrder("", "session2", dyingUser.Email, "USD")
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
			test.DB.Create(i)
		}
		defer func() {
			for _, i := range items {
				test.DB.Unscoped().Delete(i)
			}
		}()

		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodDelete, "/users/"+dyingUser.ID, nil, token)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "", recorder.Body.String())

		// now load it back and it should be soft deleted
		//found := &models.User{ID: dyingUser.ID}
		assert.False(t, test.DB.Unscoped().First(&dyingUser).RecordNotFound())
		assert.NotNil(t, dyingUser.DeletedAt, "user wasn't deleted")
		assert.False(t, test.DB.Unscoped().First(&dyingAddr).RecordNotFound())
		assert.NotNil(t, dyingAddr.DeletedAt, "addr wasn't deleted")
		assert.False(t, test.DB.Unscoped().First(dyingOrder).RecordNotFound())
		assert.NotNil(t, dyingOrder.DeletedAt, "order wasn't deleted")
		assert.False(t, test.DB.Unscoped().First(&dyingTransaction).RecordNotFound())
		assert.NotNil(t, dyingTransaction.DeletedAt, "transaction wasn't deleted")
		assert.False(t, test.DB.Unscoped().First(&dyingLineItem).RecordNotFound())
		assert.NotNil(t, dyingLineItem.DeletedAt, "line item wasn't deleted")
	})
}

func TestUserAddressDelete(t *testing.T) {
	test := NewRouteTest(t)
	addr := getTestAddress()
	addr.UserID = test.Data.testUser.ID
	test.DB.Create(addr)

	token := testAdminToken("magical-unicorn", "")
	recorder := test.TestEndpoint(http.MethodDelete, "/users/"+test.Data.testUser.ID+"/addresses/"+addr.ID, nil, token)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "", recorder.Body.String())

	assert.False(t, test.DB.Unscoped().First(&addr).RecordNotFound())
	assert.NotNil(t, addr.DeletedAt)
}

func TestUserAddressCreate(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		test := NewRouteTest(t)
		addr := getTestAddress()
		b, err := json.Marshal(&addr.AddressRequest)
		require.NoError(t, err)

		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodPost, "/users/"+test.Data.testUser.ID+"/addresses", bytes.NewBuffer(b), token)

		results := struct {
			ID string
		}{}
		extractPayload(t, http.StatusOK, recorder, &results)

		// now pull off the address from the DB
		dbAddr := &models.Address{ID: results.ID, UserID: test.Data.testUser.ID}
		rsp := test.DB.First(dbAddr)
		assert.False(t, rsp.RecordNotFound())
	})
	t.Run("Invalid", func(t *testing.T) {
		test := NewRouteTest(t)
		addr := getTestAddress()
		addr.Name = "" // required field
		b, err := json.Marshal(&addr.AddressRequest)
		require.NoError(t, err)

		token := testAdminToken("magical-unicorn", "")
		recorder := test.TestEndpoint(http.MethodPost, "/users/"+test.Data.testUser.ID+"/addresses", bytes.NewBuffer(b), token)
		validateError(t, http.StatusBadRequest, recorder)
	})
}
