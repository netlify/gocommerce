package api

import (
	"context"
	"net/http"

	"encoding/json"
	"fmt"

	"github.com/guregu/kami"
	"github.com/netlify/netlify-commerce/models"
)

func (a *API) DiscountList(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	log, _, httpErr := requireAdmin(ctx, "")
	if httpErr != nil {
		log.WithError(httpErr).Warn("Illegal access attempt for discount list")
		sendJSON(w, httpErr.Code, httpErr)
		return
	}

	log.Debug("Requesting a list of the discounts")
	discounts := []models.Discount{}
	if res := a.db.Find(&discounts); res.Error != nil {
		log.WithError(res.Error).Warn("Failed to query for all the discounts")
		internalServerError(w, "Failed to query for discounts")
		return
	}

	log.Debug("Finished fetching %d discounts", len(discounts))
	out := []models.DiscountExternal{}
	for _, d := range discounts {
		out = append(out, d.Externalize())
	}

	sendJSON(w, http.StatusOK, out)
}

func (a *API) DiscountCreate(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	log, _, httpErr := requireAdmin(ctx, "")
	if httpErr != nil {
		log.WithError(httpErr).Warn("Illegal access attempt for discount create")
		sendJSON(w, httpErr.Code, httpErr)
		return
	}

	params := &models.DiscountExternal{}
	if err := json.NewDecoder(r.Body).Decode(params); err != nil {
		badRequestError(w, fmt.Sprintf("Could not read params: %v", err))
		return
	}

	if err := params.Validate(); err != nil {
		badRequestError(w, "Request is invalid because of: %v", err)
		return
	}

	discount := params.Internalize()
	if rsp := a.db.Create(discount); rsp.Error != nil {
		internalServerError(w, "Failed to save discount")
		log.WithError(rsp.Error).Warn("Failed to save discount")
		return
	}

	sendJSON(w, http.StatusNoContent, nil)
}

func (a *API) DiscountDelete(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	log, discountID, httpErr := requireAdmin(ctx, "discount_id")
	if httpErr != nil {
		log.WithError(httpErr).Warn("Illegal access attempt for discount delete")
		sendJSON(w, httpErr.Code, httpErr)
		return
	}

	dis := &models.Discount{ID: discountID}
	if rsp := a.db.Delete(&dis); rsp.Error != nil {
		if rsp.RecordNotFound() {
			notFoundError(w, "Failed to find discount for id %s", discountID)
			return
		}

		log.WithError(rsp.Error).Warn("Failed to query for discount")
		internalServerError(w, "Failed to query for discount %s", discountID)
		return
	}

	sendJSON(w, http.StatusNoContent, nil)
}

func (a *API) DiscountView(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	log := getLogger(ctx)
	id := kami.Param(ctx, "discount_id")

	d := new(models.Discount)
	if rsp := a.db.Where("id = ?", id).First(d); rsp.Error != nil {
		if rsp.RecordNotFound() {
			notFoundError(w, "Failed to find discount for id %s", id)
			return
		}
		log.WithError(rsp.Error).Warn("Failed to query for discount")
		internalServerError(w, "Failed to query for discount %s", id)
		return
	}

	sendJSON(w, http.StatusOK, d.Externalize())
}
