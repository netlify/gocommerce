package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/guregu/kami"
	"github.com/stretchr/testify/assert"

	"github.com/netlify/netlify-commerce/models"
)

// ------------------------------------------------------------------------------------------------
// List by ORDER
// ------------------------------------------------------------------------------------------------

func TestPaymentsOrderForAllAsOwner(t *testing.T) {
	db, config := db(t)

	ctx := testContext(testToken(testUser.ID, ""), config, false)
	ctx = kami.SetParam(ctx, "order_id", firstOrder.ID)
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", nil)

	NewAPI(config, db, nil).PaymentListForOrder(ctx, w, r)

	// we should have gotten back a list of transactions
	trans := []models.Transaction{}
	extractPayload(t, 200, w, &trans)
	assert.Equal(t, 1, len(trans))
	validateTransaction(t, firstTransaction, &trans[0])
}

func TestPaymentsOrderQueryForAllAsAdmin(t *testing.T) {
	db, config := db(t)
	anotherTransaction := models.NewTransaction(firstOrder)
	db.Create(anotherTransaction)
	defer db.Unscoped().Delete(anotherTransaction)

	ctx := testContext(testToken("magical-unicorn", ""), config, true)
	ctx = kami.SetParam(ctx, "order_id", firstOrder.ID)
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", nil)

	NewAPI(config, db, nil).PaymentListForOrder(ctx, w, r)

	// we should have gotten back a list of transactions
	trans := []models.Transaction{}
	extractPayload(t, 200, w, &trans)
	assert.Equal(t, 2, len(trans))

	for _, tran := range trans {
		switch tran.ID {
		case anotherTransaction.ID:
			validateTransaction(t, anotherTransaction, &tran)
		case firstTransaction.ID:
			validateTransaction(t, firstTransaction, &tran)
		default:
			assert.Fail(t, "Unknown transaction: "+tran.ID)
		}
	}
}

func TestPaymentsOrderQueryForAllAsAnon(t *testing.T) {
	db, config := db(t)

	ctx := testContext(nil, config, false)
	ctx = kami.SetParam(ctx, "order_id", firstOrder.ID)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", nil)
	NewAPI(config, db, nil).PaymentListForOrder(ctx, w, r)

	// should get a 401 ~ claims are required
	validateError(t, 401, w)
}

// ------------------------------------------------------------------------------------------------
// Validators
// ------------------------------------------------------------------------------------------------
func validateTransaction(t *testing.T, expected *models.Transaction, actual *models.Transaction) {
	assert := assert.New(t)
	assert.Equal(expected.Currency, actual.Currency)
	assert.Equal(expected.ID, actual.ID)
	assert.Equal(expected.OrderID, actual.OrderID)
	assert.Equal(expected.ProcessorID, actual.ProcessorID)
	assert.Equal(expected.UserID, actual.UserID)
	assert.Equal(expected.Amount, actual.Amount)
	assert.Equal(expected.AmountReversed, actual.AmountReversed)
	assert.Equal(expected.Currency, actual.Currency)
	assert.Equal(expected.FailureCode, actual.FailureCode)
	assert.Equal(expected.FailureDescription, actual.FailureDescription)
	assert.Equal(expected.Status, actual.Status)
	assert.Equal(expected.Type, actual.Type)
	assert.Equal(expected.CreatedAt.UTC(), actual.CreatedAt.UTC())
}
