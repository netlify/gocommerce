package api

import (
	"context"
	"net/http"

	"github.com/netlify/gocommerce/models"
)

type SalesRow struct {
	Total    uint64 `json:"total"`
	SubTotal uint64 `json:"subtotal"`
	Taxes    uint64 `json:"taxes"`
	Currency string `json:"currency"`
}

type ProductsRow struct {
	Sku      string `json:"sku"`
	Path     string `json:"path"`
	Total    uint64 `json:"total"`
	Currency string `json:"currency"`
}

// SalesReport lists the sales numbers for a period
func (a *API) SalesReport(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	query := a.db.
		Model(&models.Order{}).
		Select("sum(total) as total, sum(sub_total) as subtotal, sum(taxes) as taxes, currency").
		Where("payment_state = 'paid'").
		Group("currency")

	query, err := parseTimeQueryParams(query, r.URL.Query())
	if err != nil {
		badRequestError(w, err.Error())
		return
	}

	rows, err := query.Rows()
	if err != nil {
		internalServerError(w, "Database error: %v", err)
		return
	}
	defer rows.Close()
	result := []*SalesRow{}
	for rows.Next() {
		row := &SalesRow{}
		err = rows.Scan(&row.Total, &row.SubTotal, &row.Taxes, &row.Currency)
		if err != nil {
			internalServerError(w, "Database error: %v", err)
			return
		}
		result = append(result, row)
	}

	sendJSON(w, 200, result)
}

// ProductsReport list the products sold within a period
func (a *API) ProductsReport(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	ordersTable := models.Order{}.TableName()
	itemsTable := models.LineItem{}.TableName()
	query := a.db.
		Model(&models.LineItem{}).
		Select("sku, path, sum(quantity * price) as total, currency").
		Joins("JOIN " + ordersTable + " as orders " + "ON orders.id = " + itemsTable + ".order_id " + "AND orders.payment_state = 'paid'").
		Group("sku, path, currency").
		Order("total desc")

	from, to, err := getTimeQueryParams(r.URL.Query())
	if err != nil {
		badRequestError(w, err.Error())
		return
	}
	if from != nil {
		query = query.Where("orders.created_at >= ?", from)
	}
	if to != nil {
		query.Where("orders.created_at <= ?", to)
	}

	rows, err := query.Rows()
	if err != nil {
		internalServerError(w, "Database error: %v", err)
		return
	}
	defer rows.Close()
	result := []*ProductsRow{}
	for rows.Next() {
		row := &ProductsRow{}
		err = rows.Scan(&row.Sku, &row.Path, &row.Total, &row.Currency)
		if err != nil {
			internalServerError(w, "Database error: %v", err)
			return
		}
		result = append(result, row)
	}

	sendJSON(w, 200, result)
}
