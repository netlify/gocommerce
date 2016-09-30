package api

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/jinzhu/gorm"
)

func parsePaymentQueryParams(query *gorm.DB, params url.Values) (*gorm.DB, error) {
	query = addFilters(query, params, []string{
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
	query = addFilters(query, params, []string{
		"id",
		"email",
	})

	query, err := parseLimitQueryParam(query, params)
	if err != nil {
		return nil, err
	}
	return parseTimeQueryParams(query, params)
}

func parseOrderParams(query *gorm.DB, params url.Values) (*gorm.DB, error) {
	if values, exists := params["sort"]; exists {
		switch values[0] {
		case "desc":
			query = query.Order("created_at DESC")
		case "asc":
			query = query.Order("created_at ASC")
		default:
			return nil, fmt.Errorf("bad value for 'sort' parameter: only 'asc' or 'desc' are accepted")
		}
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

func addFilters(query *gorm.DB, params url.Values, availableFilters []string) *gorm.DB {
	for _, filter := range availableFilters {
		if values, exists := params[filter]; exists {
			query = query.Where(filter+" = ?", values[0])
		}
	}
	return query
}
