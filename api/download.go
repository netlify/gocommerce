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
func (a *API) DownloadURL(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	downloadID := chi.URLParam(r, "download_id")
	logEntrySetField(r, "download_id", downloadID)
	claims := gcontext.GetClaims(ctx)
	assets := gcontext.GetAssetStore(ctx)

	download := &models.Download{}
	if result := a.db.Where("id = ?", downloadID).First(download); result.Error != nil {
		if result.RecordNotFound() {
			return notFoundError("Download not found")
		}
		return internalServerError("Error during database query").WithInternalError(result.Error)
	}

	order := &models.Order{}
	if result := a.db.Where("id = ?", download.OrderID).First(order); result.Error != nil {
		if result.RecordNotFound() {
			return notFoundError("Download order not found")
		}
		return internalServerError("Error during database query").WithInternalError(result.Error)
	}

	if !hasOrderAccess(ctx, order) {
		return unauthorizedError("Not Authorized to access this download")
	}

	if order.PaymentState != models.PaidState {
		return unauthorizedError("This download has not been paid yet")
	}

	rows, err := a.db.Model(&models.Event{}).
		Select("count(distinct(ip))").
		Where("order_id = ? and created_at > ? and changes = 'download'", order.ID, time.Now().Add(-24*time.Hour)).
		Rows()
	if err != nil {
		return internalServerError("Error signing download").WithInternalError(err)
	}
	var count uint64
	for rows.Next() {
		err = rows.Scan(&count)
		if err != nil {
			return internalServerError("Error signing download").WithInternalError(err)
		}
	}
	if count > maxIPsPerDay {
		return unauthorizedError("This download has been accessed from too many IPs within the last day")
	}

	if err := download.SignURL(assets); err != nil {
		return internalServerError("Error signing download").WithInternalError(err)
	}

	tx := a.db.Begin()
	tx.Model(download).Updates(map[string]interface{}{"download_count": gorm.Expr("download_count + 1")})
	var subject string
	if claims != nil {
		subject = claims.Subject
	}
	models.LogEvent(tx, r.RemoteAddr, subject, order.ID, models.EventUpdated, []string{"download"})
	tx.Commit()

	return sendJSON(w, http.StatusOK, download)
}

// DownloadList lists all purchased downloads for an order or a user.
func (a *API) DownloadList(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	orderID := gcontext.GetOrderID(ctx)
	log := getLogEntry(r)
	claims := gcontext.GetClaims(ctx)

	order := &models.Order{}
	if orderID != "" {
		if result := a.db.Where("id = ?", orderID).First(order); result.Error != nil {
			if result.RecordNotFound() {
				return notFoundError("Download order not found")
			}
			return internalServerError("Error during database query").WithInternalError(result.Error)
		}
	} else {
		order = nil
	}

	if order != nil {
		if !hasOrderAccess(ctx, order) {
			return unauthorizedError("You don't have permission to access this order")
		}

		if order.PaymentState != models.PaidState {
			return unauthorizedError("This order has not been completed yet")
		}
	}

	orderTable := a.db.NewScope(models.Order{}).QuotedTableName()
	downloadsTable := a.db.NewScope(models.Download{}).QuotedTableName()

	query := a.db.Joins("join " + orderTable + " as orders ON " + downloadsTable + ".order_id = orders.id and orders.payment_state = 'paid'")
	if order != nil {
		query = query.Where("orders.id = ?", order.ID)
	} else {
		query = query.Where("orders.user_id = ?", claims.Subject)
	}

	offset, limit, err := paginate(w, r, query.Model(&models.Download{}))
	if err != nil {
		return badRequestError("Bad Pagination Parameters: %v", err)
	}

	var downloads []models.Download
	if result := query.Offset(offset).Limit(limit).Find(&downloads); result.Error != nil {
		return internalServerError("Error during database query").WithInternalError(err)
	}

	log.WithField("download_count", len(downloads)).Debugf("Successfully retrieved %d downloads", len(downloads))
	return sendJSON(w, http.StatusOK, downloads)
}
