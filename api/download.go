package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/jinzhu/gorm"
	gcontext "github.com/netlify/gocommerce/context"
	"github.com/netlify/gocommerce/models"
)

const maxIPsPerDay = 50

// DownloadURL returns a signed URL to download a purchased asset.
func (a *API) DownloadURL(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	downloadID := chi.URLParam(r, "download_id")
	log := logEntrySetField(r, "download_id", downloadID)
	claims := gcontext.GetClaims(ctx)
	assets := gcontext.GetAssetStore(ctx)

	download := &models.Download{}
	if result := a.db.Where("id = ?", downloadID).First(download); result.Error != nil {
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
	if result := a.db.Where("id = ?", download.OrderID).First(order); result.Error != nil {
		if result.RecordNotFound() {
			log.Debug("Requested record that doesn't exist")
			notFoundError(w, "Download order not found")
		} else {
			log.WithError(result.Error).Warnf("Error while querying database: %s", result.Error.Error())
			internalServerError(w, "Error during database query: %v", result.Error)
		}
		return
	}

	if !hasOrderAccess(ctx, order) {
		unauthorizedError(w, "Not Authorized to access this download")
		return
	}

	if order.PaymentState != "paid" {
		unauthorizedError(w, "This download has not been paid yet")
		return
	}

	rows, err := a.db.Model(&models.Event{}).
		Select("count(distinct(ip))").
		Where("order_id = ? and created_at > ? and changes = 'download'", order.ID, time.Now().Add(-24*time.Hour)).
		Rows()
	if err != nil {
		log.WithError(err).Warnf("Error while signing download: %s", err)
		internalServerError(w, "Error signing download: %v", err)
		return
	}
	var count uint64
	for rows.Next() {
		err = rows.Scan(&count)
		if err != nil {
			log.WithError(err).Warnf("Error while signing download: %s", err)
			internalServerError(w, "Error signing download: %v", err)
			return
		}
	}
	if count > maxIPsPerDay {
		unauthorizedError(w, "This download has been accessed from too many IPs within the last day")
		return
	}

	if err := download.SignURL(assets); err != nil {
		log.WithError(err).Warnf("Error while signing download: %s", err)
		internalServerError(w, "Error signing download: %v", err)
		return
	}

	tx := a.db.Begin()
	tx.Model(download).Updates(map[string]interface{}{"download_count": gorm.Expr("download_count + 1")})
	models.LogEvent(tx, r.RemoteAddr, claims.ID, order.ID, models.EventUpdated, []string{"download"})
	tx.Commit()

	sendJSON(w, http.StatusOK, download)
}

// DownloadList lists all purchased downloads for an order or a user.
func (a *API) DownloadList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orderID := getOrderID(ctx)
	log := getLogEntry(r)
	claims := gcontext.GetClaims(ctx)

	order := &models.Order{}
	if orderID != "" {
		if result := a.db.Where("id = ?", orderID).First(order); result.Error != nil {
			if result.RecordNotFound() {
				log.Debug("Requested record that doesn't exist")
				notFoundError(w, "Download order not found")
			} else {
				log.WithError(result.Error).Warnf("Error while querying database: %s", result.Error.Error())
				internalServerError(w, "Error during database query: %v", result.Error)
			}
			return
		}
	} else {
		order = nil
	}

	if order != nil {
		if !hasOrderAccess(ctx, order) {
			unauthorizedError(w, "You don't have permission to access this order")
			return
		}

		if order.PaymentState != "paid" {
			unauthorizedError(w, "This order has not been completed yet")
			return
		}
	}

	orderTable := models.Order{}.TableName()
	downloadsTable := models.Download{}.TableName()

	query := a.db.Joins("join " + orderTable + " as orders ON " + downloadsTable + ".order_id = orders.id and orders.payment_state = 'paid'")
	if order != nil {
		query = query.Where("orders.id = ?", order.ID)
	} else {
		query = query.Where("orders.user_id = ?", claims.ID)
	}

	offset, limit, err := paginate(w, r, query.Model(&models.Download{}))
	if err != nil {
		badRequestError(w, "Bad Pagination Parameters: %v", err)
		return
	}

	var downloads []models.Download
	query.LogMode(true)
	if result := query.Offset(offset).Limit(limit).Find(&downloads); result.Error != nil {
		log.WithError(result.Error).Warn("Error while querying database")
		internalServerError(w, "Error during database query: %v", result.Error)
		return
	}

	log.WithField("download_count", len(downloads)).Debugf("Successfully retrieved %d downloads", len(downloads))
	sendJSON(w, http.StatusOK, downloads)
}
