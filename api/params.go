package api

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/netlify/gocommerce/models"
)

type sortDirection string

const ascending sortDirection = "asc"
const descending sortDirection = "desc"

var sortFields = map[string]string{
	"created_at": "created_at",
	"updated_at": "updated_at",
	"email":      "email",
	"taxes":      "taxes",
	"subtotal":   "subtotal",
	"total":      "total",
}

func parsePaymentQueryParams(query *gorm.DB, params url.Values) (*gorm.DB, error) {
	query = addFilters(query, query.NewScope(models.Transaction{}).QuotedTableName(), params, []string{
		"processor_id",
		"user_id",
		"order_id",
		"failure_code",
		"currency",
		"type",
		"status",
	})

	if values, exists := params["min_amount"]; exists {
		query = query.Where("amount >= ?", values[0])
	}

	if values, exists := params["max_amount"]; exists {
		query = query.Where("amount <= ?", values[0])
	}

	query, err := parseLimitQueryParam(query, params)
	if err != nil {
		return nil, err
	}
	return parseTimeQueryParams(query, params)
}

func parseUserQueryParams(query *gorm.DB, params url.Values) (*gorm.DB, error) {
	userTable := query.NewScope(models.User{}).QuotedTableName()
	query = addFilters(query, userTable, params, []string{
		"id",
	})

	query = addLikeFilters(query, userTable, params, []string{
		"email",
	})

	query, err := parseLimitQueryParam(query, params)
	if err != nil {
		return nil, err
	}
	return parseTimeQueryParams(query, params)
}

func sortField(value string) string {
	return sortFields[value]
}

func addAddressFilter(query *gorm.DB, params url.Values, queryField string, dbField string) *gorm.DB {
	addressTable := query.NewScope(models.Address{}).QuotedTableName()
	orderTable := query.NewScope(models.Order{}).QuotedTableName()

	if billingField := params.Get("billing_" + queryField); billingField != "" {
		statement := "JOIN " + addressTable + " as billing_address on billing_address.id = " +
			orderTable + ".billing_address_id AND " + "billing_address." + dbField + " in (?)"
		query = query.Joins(statement, strings.Split(billingField, ","))
	}

	if shippingField := params.Get("shipping_" + queryField); shippingField != "" {
		statement := "JOIN " + addressTable + " as shipping_address on shipping_address.id = " +
			orderTable + ".shipping_address_id AND " + "shipping_address." + dbField + " in (?)"
		query = query.Joins(statement, strings.Split(shippingField, ","))
	}
	return query
}

func parseOrderParams(query *gorm.DB, params url.Values) (*gorm.DB, error) {
	if tax := params.Get("tax"); tax != "" {
		if tax == "yes" || tax == "true" {
			query = query.Where("taxes > 0")
		} else {
			query = query.Where("taxes = 0")
		}
	}

	if invoiceNumber := params.Get("invoice_number"); invoiceNumber != "" {
		query = query.Where("invoice_number = ?", invoiceNumber)
	}

	orderTable := query.NewScope(models.Order{}).QuotedTableName()

	query = addAddressFilter(query, params, "countries", "country")
	query = addAddressFilter(query, params, "name", "name")

	if values, exists := params["sort"]; exists {
		for _, value := range values {
			parts := strings.Split(value, " ")
			field := sortField(parts[0])
			if field == "" {
				return nil, fmt.Errorf("bad field for sort '%v'", field)
			}
			dir := ascending
			if len(parts) == 2 {
				switch strings.ToLower(parts[1]) {
				case string(ascending):
					dir = ascending
				case string(descending):
					dir = descending
				default:
					return nil, fmt.Errorf("bad direction for sort '%v', only 'asc' and 'desc' allowed", parts[1])
				}
			}
			query = query.Order(field + " " + string(dir))
		}
	} else {
		query = query.Order("created_at desc")
	}

	if email := params.Get("email"); email != "" {
		query = query.Where(orderTable+".email LIKE ?", "%"+email+"%")
	}

	if items := params.Get("items"); items != "" {
		lineItemTable := query.NewScope(models.LineItem{}).QuotedTableName()
		statement := "JOIN " + lineItemTable + " as line_item on line_item.order_id = " +
			orderTable + ".id AND line_item.title LIKE ?"
		query = query.Joins(statement, "%"+items+"%")
	}

	if itemType := params.Get("item_type"); itemType != "" {
		lineItemTable := query.NewScope(models.LineItem{}).QuotedTableName()
		statement := "JOIN " + lineItemTable + " as line_item on line_item.order_id = " +
			orderTable + ".id AND line_item.type LIKE ?"
		query = query.Joins(statement, "%"+itemType+"%")
	}

	if code := params.Get("coupon_code"); code != "" {
		query = query.Where(orderTable+".coupon_code LIKE ?", "%"+code+"%")
	}

	return parseTimeQueryParams(query, params)
}

func parseLimitQueryParam(query *gorm.DB, params url.Values) (*gorm.DB, error) {
	if values, exists := params["limit"]; exists {
		v, err := strconv.Atoi(values[0])
		if err != nil {
			return nil, err
		}
		query = query.Limit(v)
	}

	return query, nil
}

func getTimeQueryParams(params url.Values) (from *time.Time, to *time.Time, err error) {
	if value := params.Get("from"); value != "" {
		ts, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return from, to, fmt.Errorf("bad value for 'from' parameter: %s", err)
		}
		t := time.Unix(ts, 0)
		from = &t
	}

	if value := params.Get("to"); value != "" {
		ts, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return from, to, fmt.Errorf("bad value for 'to' parameter: %s", err)
		}
		t := time.Unix(ts, 0)
		to = &t
	}
	return
}

func parseTimeQueryParams(query *gorm.DB, params url.Values) (*gorm.DB, error) {
	from, to, err := getTimeQueryParams(params)
	if err != nil {
		return nil, err
	}
	if from != nil {
		query = query.Where("created_at >= ?", from)
	}
	if to != nil {
		query = query.Where("created_at <= ?", to)
	}
	return query, nil
}

func addFilters(query *gorm.DB, table string, params url.Values, availableFilters []string) *gorm.DB {
	for _, filter := range availableFilters {
		if values, exists := params[filter]; exists {
			query = query.Where(table+"."+filter+" = ?", values[0])
		}
	}
	return query
}

func addLikeFilters(query *gorm.DB, table string, params url.Values, availableFilters []string) *gorm.DB {
	for _, filter := range availableFilters {
		if values, exists := params[filter]; exists {
			query = query.Where(table+"."+filter+" LIKE ?", "%"+values[0]+"%")
		}
	}
	return query
}
