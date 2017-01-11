package api

import (
	"context"
	"net/http"

	"github.com/guregu/kami"
	"github.com/jinzhu/gorm"
	"github.com/netlify/netlify-commerce/models"
)

func (a *API) DownloadURL(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	id := kami.Param(ctx, "id")
	log := getLogger(ctx).WithField("download_id", id)
	claims := getClaims(ctx)

	download := &models.Download{}
	if result := a.db.Where("id = ?", id).First(download); result.Error != nil {
		if result.RecordNotFound() {
			log.Debug("Requested record that doesn't exist")
			notFoundError(w, "Download not found")
		} else {
			log.WithError(result.Error).Warnf("Error while querying database: %s", result.Error.Error())
			internalServerError(w, "Error during database query: %v", result.Error)
		}
		return
	}

	order := &models.Order{}
	if result := a.db.Where("id = ?", id).First(order); result.Error != nil {
		if result.RecordNotFound() {
			log.Debug("Requested record that doesn't exist")
			notFoundError(w, "Download order not found")
		} else {
			log.WithError(result.Error).Warnf("Error while querying database: %s", result.Error.Error())
			internalServerError(w, "Error during database query: %v", result.Error)
		}
		return
	}

	if order.UserID != "" {
		if (order.UserID != claims.ID) && isAdmin(ctx) {
			unauthorizedError(w, "Not Authorized to access this download")
			return
		}
	}

	if order.PaymentState != "paid" {
		unauthorizedError(w, "This download has not been paid yet")
		return
	}

	if err := download.SignURL(a.assets); err != nil {
		log.WithError(err).Warnf("Error while signing download: %s", err)
		internalServerError(w, "Error signing download: %v", err)
		return
	}

	tx := a.db.Begin()
	tx.Model(download).Updates(map[string]interface{}{"download_count": gorm.Expr("download_count + 1")})
	models.LogEvent(tx, claims.ID, order.ID, models.EventUpdated, []string{"download"})

	sendJSON(w, 200, download)
}
