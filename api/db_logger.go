package api

import (
	"context"
	"net/http"

	"github.com/jinzhu/gorm"
	gcontext "github.com/netlify/gocommerce/context"
	"github.com/netlify/gocommerce/models"
)

func (a *API) loggingDB(w http.ResponseWriter, r *http.Request) (context.Context, error) {
	if a.db == nil {
		return r.Context(), nil
	}

	log := getLogEntry(r)
	db := a.db.New()
	db.SetLogger(models.NewDBLogger(log))

	return gcontext.WithDB(r.Context(), db), nil
}

// DB provides callers with a database instance configured for request logging
func (a *API) DB(r *http.Request) *gorm.DB {
	ctx := r.Context()
	return gcontext.GetDB(ctx)
}
