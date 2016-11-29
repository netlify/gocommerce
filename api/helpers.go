package api

import (
	"encoding/json"
	"net/http"
)

func sendJSON(w http.ResponseWriter, status int, obj interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if obj != nil {
		encoder := json.NewEncoder(w)
		encoder.Encode(obj)
	}
}
