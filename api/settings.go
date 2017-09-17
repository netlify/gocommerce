package api

import (
	"fmt"
	"io"
	"net/http"

	gcontext "github.com/netlify/gocommerce/context"
)

func (a *API) ViewSettings(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	config := gcontext.GetConfig(ctx)

	resp, err := a.httpClient.Get(config.SettingsURL())
	if err != nil {
		return fmt.Errorf("Error loading site settings: %v", err)
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err = io.Copy(w, resp.Body)
	return err
}
