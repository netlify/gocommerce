package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jinzhu/gorm"
)

const defaultPerPage = 50

func calculateTotalPages(perPage, total uint64) uint64 {
	pages := total / perPage
	if total%perPage > 0 {
		return pages + 1
	}
	return pages
}

func addPaginationHeaders(w http.ResponseWriter, r *http.Request, page, perPage, total uint64) {
	totalPages := calculateTotalPages(perPage, total)
	url, _ := url.ParseRequestURI(r.URL.String())
	query := url.Query()
	header := ""
	if totalPages > page {
		query.Set("page", fmt.Sprintf("%v", page+1))
		url.RawQuery = query.Encode()
		header += "<" + url.String() + ">; rel=\"next\", "
	}
	query.Set("page", fmt.Sprintf("%v", totalPages))
	url.RawQuery = query.Encode()
	header += "<" + url.String() + ">; rel=\"last\""

	w.Header().Add("Link", header)
	w.Header().Add("X-Total-Count", fmt.Sprintf("%v", total))
}

func paginate(w http.ResponseWriter, r *http.Request, query *gorm.DB) (offset int, limit int, err error) {
	params := r.URL.Query()
	queryPage := params.Get("page")
	queryPerPage := params.Get("per_page")
	var page uint64 = 1
	var perPage uint64 = defaultPerPage
	if queryPage != "" {
		page, err = strconv.ParseUint(queryPage, 10, 64)
		if err != nil {
			return
		}
	}
	if queryPerPage != "" {
		perPage, err = strconv.ParseUint(queryPerPage, 10, 64)
		if err != nil {
			return
		}
	}

	var total uint64
	if result := query.Count(&total); result.Error != nil {
		err = result.Error
		return
	}

	offset = int((page - 1) * perPage)
	limit = int(perPage)
	addPaginationHeaders(w, r, page, perPage, total)

	return
}
