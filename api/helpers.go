package api

import (
	"encoding/json"
	"net/http"

	"github.com/Sirupsen/logrus"
)

func sendJSON(w http.ResponseWriter, status int, obj interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(obj); err != nil {
		logrus.WithError(err).Errorf("Error encoding json response: %v", obj)
	}
}
