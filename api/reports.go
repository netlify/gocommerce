package api

import (
	"net/http"

	"github.com/netlify/gocommerce/models"
)

type salesRow struct {
	Total    uint64 `json:"total"`
	SubTotal uint64 `json:"subtotal"`
	Taxes    uint64 `json:"taxes"`
	Currency string `json:"currency"`
}

type productsRow struct {
	Sku      string `json:"sku"`
	Path     string `json:"path"`
	Total    uint64 `json:"total"`
	Currency string `json:"currency"`
}

// SalesReport lists the sales numbers for a period
func (a *API) SalesReport(w http.ResponseWriter, r *http.Request) error {
	query := a.db.
		Model(&models.Order{}).
		Select("sum(total) as total, sum(sub_total) as subtotal, sum(taxes) as taxes, currency").
		Where("payment_state = 'paid'").
		Group("currency")

	query, err := parseTimeQueryParams(query, r.URL.Query())
	if err != nil {
		return badRequestError(err.Error())
	}

	rows, err := query.Rows()
	if err != nil {
		return internalServerError("Database error: %v", err)
	}
	defer rows.Close()
	result := []*salesRow{}
	for rows.Next() {
		row := &salesRow{}
		err = rows.Scan(&row.Total, &row.SubTotal, &row.Taxes, &row.Currency)
		if err != nil {
			return internalServerError("Database error: %v", err)
		}
		result = append(result, row)
	}

	return sendJSON(w, http.StatusOK, result)
}

// ProductsReport list the products sold within a period
func (a *API) ProductsReport(w http.ResponseWriter, r *http.Request) error {
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
		return badRequestError(err.Error())
	}
	if from != nil {
		query = query.Where("orders.created_at >= ?", from)
	}
	if to != nil {
		query.Where("orders.created_at <= ?", to)
	}

	rows, err := query.Rows()
	if err != nil {
		return internalServerError("Database error: %v", err)
	}
	defer rows.Close()
	result := []*productsRow{}
	for rows.Next() {
		row := &productsRow{}
		err = rows.Scan(&row.Sku, &row.Path, &row.Total, &row.Currency)
		if err != nil {
			return internalServerError("Database error: %v", err)
		}
		result = append(result, row)
	}

	return sendJSON(w, http.StatusOK, result)
}
