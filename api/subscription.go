package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	gcontext "github.com/netlify/gocommerce/context"
	"github.com/netlify/gocommerce/models"
)

func (a *API) processSubscription(ctx context.Context, sub *models.SubscriptionItem) error {
	config := gcontext.GetConfig(ctx)
	metaTags, err := a.extractMetadata(config.SiteURL+sub.Path, ".gocommerce-service")
	if err != nil {
		return err
	}
	if len(metaTags) == 0 {
		return fmt.Errorf("No script tag with class gocommerce-product tag found for '%v'", sub.Sku)
	}

	metaProducts := make([]*models.SubscriptionMetadata, 0, len(metaTags))
	for _, metaJSON := range metaTags {
		meta := new(models.SubscriptionMetadata)
		if err := json.Unmarshal([]byte(metaJSON), meta); err != nil {
			return err
		}
		metaProducts = append(metaProducts, meta)
	}

	if len(metaProducts) == 1 && sub.Sku == "" {
		sub.Sku = metaProducts[0].Sku
	}

	for _, meta := range metaProducts {
		if meta.Sku == sub.Sku {
			return sub.Process(meta)
		}
	}

	return fmt.Errorf("No product Sku from path matched: %v", sub.Sku)
}

type subscriptionRequestParams struct {
	Sku      string `json:"sku"`
	Path     string `json:"path"`
	Quantity uint64 `json:"quantity"`
	Currency string `json:"currency"`

	Metadata map[string]interface{} `json:"metadata"`
}

// SubscriptionCreate subscribes a user to a plan
func (a *API) SubscriptionCreate(w http.ResponseWriter, r *http.Request) error {
	log := getLogEntry(r)
	ctx := r.Context()
	config := gcontext.GetConfig(ctx)
	instanceID := gcontext.GetInstanceID(ctx)
	var err error

	claims := gcontext.GetClaims(ctx)
	if claims == nil || claims.Subject == "" {
		return badRequestError("Subscriptions can only be created by a user with a valid JWT")
	}
	user, err := userFromClaims(a.db, claims, log)
	if err != nil {
		return err
	}

	params := &subscriptionRequestParams{Currency: "USD", Quantity: 1}
	jsonDecoder := json.NewDecoder(r.Body)
	err = jsonDecoder.Decode(params)
	if err != nil {
		return badRequestError("Could not read params: %v", err)
	}

	sub := &models.SubscriptionItem{
		InstanceID:   instanceID,
		UserID:       user.ID,
		Sku:          params.Sku,
		Path:         params.Path,
		Quantity:     params.Quantity,
		Currency:     params.Currency,
		MetaData:     params.Metadata,
		PaymentState: models.SubscriptionPending,
	}
	if err := a.processSubscription(ctx, sub); err != nil {
		return internalServerError("Error processing subscription").WithInternalError(err)
	}

	// TODO: taxes
	sub.Total = sub.Price * sub.Quantity

	// TODO: billing address

	tx := a.db.Begin()
	result := tx.Create(sub)
	if result.Error != nil {
		tx.Rollback()
		return internalServerError("failed creating subscription").WithInternalError(result.Error)
	}

	if sub.Callback != "" {
		hook, err := models.NewHook("subscription", config.SiteURL, sub.Callback, user.ID, config.Webhooks.Secret, sub)
		if err != nil {
			log.WithError(err).Error("Failed to process webhook")
		}
		tx.Save(hook)
	}
	tx.Commit()

	return sendJSON(w, http.StatusCreated, sub)
}

// TODO: payment provider handling - create plan, update subscription

// TODO: updating a subscription (with metadata)

// TODO: cancelling a subscription
