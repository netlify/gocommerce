package api

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/netlify/netlify-commerce/models"
)

var sortFields = map[string]string{
	"created_at": "created_at",
	"updated_at": "updated_at",
	"email":      "email",
	"taxes":      "taxes",
	"subtotal":   "subtotal",
	"total":      "total",
}

func parsePaymentQueryParams(query *gorm.DB, params url.Values) (*gorm.DB, error) {
	query = addFilters(query, models.Transaction{}.TableName(), params, []string{
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
	query = addFilters(query, models.User{}.TableName(), params, []string{
		"id",
		"email",
	})

	query, err := parseLimitQueryParam(query, params)
	if err != nil {
		return nil, err
	}
	return parseTimeQueryParams(query, params)
}

func sortField(value string) string {
	field, _ := sortFields[value]
	return field
}

func parseOrderParams(query *gorm.DB, params url.Values) (*gorm.DB, error) {
	if tax := params.Get("tax"); tax != "" {
		if tax == "yes" || tax == "true" {
			query = query.Where("taxes > 0")
		} else {
			query = query.Where("taxes = 0")
		}
	}

	if billingCountries := params.Get("billing_countries"); billingCountries != "" {
		addressTable := models.Address{}.TableName()
		orderTable := models.Order{}.TableName()
		statement := "JOIN " + addressTable + " as billing_address on billing_address.id = " +
			orderTable + ".billing_address_id AND " + "billing_address.country in (?)"
		query = query.Joins(statement, strings.Split(billingCountries, ","))
	}

	if shippingCountries := params.Get("shipping_countries"); shippingCountries != "" {
		addressTable := models.Address{}.TableName()
		orderTable := models.Order{}.TableName()
		statement := "JOIN " + addressTable + " as shipping_address on shipping_address.id = " +
			orderTable + ".shipping_address_id AND " + "shipping_address.country in (?)"
		query = query.Joins(statement, strings.Split(shippingCountries, ","))
	}

	if values, exists := params["sort"]; exists {
		for _, value := range values {
			parts := strings.Split(value, " ")
			field := sortField(parts[0])
			if field == "" {
				return nil, fmt.Errorf("bad field for sort '%v'", field)
			}
			dir := "asc"
			if len(parts) == 2 {
				switch strings.ToLower(parts[1]) {
				case "asc":
					dir = "asc"
				case "desc":
					dir = "desc"
				default:
					return nil, fmt.Errorf("bad direction for sort '%v', only 'asc' and 'desc' allowed", parts[1])
				}
			}
			query = query.Order(field + " " + dir)
		}
	} else {
		fmt.Println("Sorting by created_at desc")
		query = query.Order("created_at desc")
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

func parseTimeQueryParams(query *gorm.DB, params url.Values) (*gorm.DB, error) {
	if values, exists := params["from"]; exists {
		date, err := time.Parse(time.RFC3339, values[0])
		if err != nil {
			return nil, fmt.Errorf("bad value for 'from' parameter: %s", err)
		}
		query = query.Where("created_at >= ?", date)
	}

	if values, exists := params["to"]; exists {
		date, err := time.Parse(time.RFC3339, values[0])
		if err != nil {
			return nil, fmt.Errorf("bad value for 'to' parameter: %s", err)
		}
		query = query.Where("created_at <= ?", date)
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
