package api

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/jinzhu/gorm"
)

func parseUserQueryParams(query *gorm.DB, params url.Values) (*gorm.DB, error) {
	if values, exists := params["email"]; exists {
		query = query.Where("email = ?", values[0])
	}

	if values, exists := params["user_id"]; exists {
		query = query.Where("id = ?", values[0])
	}

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
