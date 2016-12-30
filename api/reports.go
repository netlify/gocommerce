package api

import (
	"context"
	"net/http"

	"github.com/netlify/netlify-commerce/models"
)

type SalesRow struct {
	Total    uint64 `json:"total"`
	SubTotal uint64 `json:"subtotal"`
	Taxes    uint64 `json:"taxes"`
	Currency string `json:"currency"`
}

// Sales lists the sales numbers for a period
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
