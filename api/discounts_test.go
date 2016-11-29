package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/guregu/kami"
	"github.com/jinzhu/gorm"
	"github.com/netlify/netlify-commerce/models"
)

func TestDiscountCreate(t *testing.T) {
	endtime := time.Date(2020, 02, 03, 04, 05, 06, 0, time.Local)
	body, err := json.Marshal(&models.DiscountExternal{
		Amount:  20,
		Type:    "percent",
		UserIDs: []string{"batman, superman"},
		ID:      "magical-unicorn",
		End:     &endtime,
	})
	if !assert.NoError(t, err) {
		return
	}

	db, config := db(t)
	ctx := testContext(testToken(testUser.ID, ""), config, true)
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", bytes.NewBuffer(body))

	NewAPI(config, db, nil).DiscountCreate(ctx, w, r)

	if assert.Equal(t, http.StatusNoContent, w.Code) {
		// validate it in the DB
		d := new(models.Discount)
		rsp := db.Find(d, "id = ?", "magical-unicorn")
		if !assert.NoError(t, rsp.Error) {
			return
		}
		validateDiscount(t, d, 20, 0, 0, "batman, superman", "", "", "percent", "magical-unicorn", nil, &endtime)
	}
}

func TestDiscountList(t *testing.T) {
	db, config := db(t)
	start, end := addDiscounts(t, db)

	ctx := testContext(testToken(testUser.ID, ""), config, true)
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", nil)

	NewAPI(config, db, nil).DiscountList(ctx, w, r)
	rspCounts := []models.DiscountExternal{}
	extractPayload(t, 200, w, &rspCounts)

	assert.Equal(t, 2, len(rspCounts))
	for _, d := range rspCounts {
		switch d.ID {
		case "fairy-dust":
			validateDiscountExt(t, &d, 10, 200, 0, []string{"red fish", "blue fish"}, nil, nil, "percent", "fairy-dust", &start, &end)
		case "marpmarp":
			validateDiscountExt(t, &d, 1234, 0, 0, nil, []string{"thing one", "thing 2"}, []string{"round", "flat"}, "flat", "marpmarp", nil, nil)
		default:
			assert.Fail(t, "Unexpected discount id: '"+d.ID+"'")
		}
	}
}

func TestDiscountDelete(t *testing.T) {
	db, config := db(t)
	addDiscounts(t, db)

	ctx := testContext(testToken(testUser.ID, ""), config, true)
	ctx = kami.SetParam(ctx, "discount_id", "fairy-dust")
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", nil)

	NewAPI(config, db, nil).DiscountDelete(ctx, w, r)

	if assert.Equal(t, http.StatusNoContent, w.Code) {
		ds := []models.Discount{}
		rsp := db.Find(&ds)
		if !assert.NoError(t, rsp.Error) {
			return
		}

		assert.Len(t, ds, 1)
		assert.Equal(t, "marpmarp", ds[0].ID)
	}
}

func TestDiscountViewExisting(t *testing.T) {
	db, config := db(t)
	start, end := addDiscounts(t, db)

	ctx := testContext(testToken(testUser.ID, ""), config, true)
	ctx = kami.SetParam(ctx, "discount_id", "fairy-dust")
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", nil)

	NewAPI(config, db, nil).DiscountView(ctx, w, r)

	dis := new(models.DiscountExternal)
	extractPayload(t, http.StatusOK, w, dis)

	validateDiscountExt(t, dis, 10, 200, 0, []string{"red fish", "blue fish"}, nil, nil, "percent", "fairy-dust", &start, &end)
}

func TestDiscountViewMissing(t *testing.T) {
	db, config := db(t)
	addDiscounts(t, db)

	ctx := testContext(testToken(testUser.ID, ""), config, true)
	ctx = kami.SetParam(ctx, "discount_id", "this-does-not-exist")
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://something", nil)

	NewAPI(config, db, nil).DiscountView(ctx, w, r)
	validateError(t, http.StatusNotFound, w)
}

// ------------------------------------------------------------------------------------------------
// Validators
// ------------------------------------------------------------------------------------------------
func addDiscounts(t *testing.T, db *gorm.DB) (time.Time, time.Time) {
	start := time.Date(2020, 10, 11, 12, 13, 14, 15, time.Local)
	end := time.Date(2040, 10, 23, 24, 25, 26, 27, time.Local)
	discounts := []models.Discount{
		{
			Amount:     10,
			OrderLimit: 200,
			UserIDs:    "red fish, blue fish",
			Start:      &start,
			End:        &end,
			Type:       "percent",
			ID:         "fairy-dust",
		},
		{
			Amount:        1234,
			Type:          "flat",
			ProductGroups: "thing one, thing 2",
			Groups:        "round, flat",
			ID:            "marpmarp",
		},
	}
	for _, d := range discounts {
		if rsp := db.Create(&d); rsp.Error != nil {
			assert.FailNow(t, "Failed to create initial discount: "+rsp.Error.Error())
		}
	}

	return start, end
}

func validateDiscount(t *testing.T, d *models.Discount, amount, limit, avail int, users, prods, groups, oType, id string, start, end *time.Time) {
	assert.Equal(t, users, d.UserIDs)
	assert.Equal(t, groups, d.Groups)
	assert.Equal(t, prods, d.ProductGroups)
	assert.Equal(t, oType, d.Type)
	assert.Equal(t, id, d.ID)
	assert.Equal(t, amount, d.Amount)
	assert.Equal(t, avail, d.Availability)
	assert.Equal(t, limit, d.OrderLimit)
	if start == nil {
		assert.Nil(t, d.Start)
	} else {
		assert.Equal(t, start.UnixNano(), d.Start.UnixNano())
	}
	if end == nil {
		assert.Nil(t, d.End)
	} else {
		assert.Equal(t, end.UnixNano(), d.End.UnixNano())
	}
}

func validateDiscountExt(t *testing.T, d *models.DiscountExternal, amount, limit, avail int, users, prods, groups []string, oType, id string, start, end *time.Time) {
	adder := func(s []string, m map[string]bool) {
		if s != nil {
			for _, v := range s {
				m[v] = true
			}
		}
	}
	act := map[string]bool{}
	adder(d.UserIDs, act)
	adder(d.ProductGroups, act)
	adder(d.Groups, act)
	exp := map[string]bool{}
	adder(users, exp)
	adder(prods, exp)
	adder(groups, exp)
	assert.Equal(t, exp, act)
	assert.Equal(t, oType, d.Type)
	assert.Equal(t, amount, d.Amount)
	assert.Equal(t, avail, d.Availability)
	assert.Equal(t, limit, d.OrderLimit)
	assert.Equal(t, id, d.ID)
	if start == nil {
		assert.Nil(t, d.Start)
	} else {
		assert.Equal(t, start.UnixNano(), d.Start.UnixNano())
	}
	if end == nil {
		assert.Nil(t, d.End)
	} else {
		assert.Equal(t, end.UnixNano(), d.End.UnixNano())
	}
}
