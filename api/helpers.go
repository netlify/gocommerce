package api

import (
	"encoding/json"
	"net/http"

	"github.com/Sirupsen/logrus"
)

func sendJSON(w http.ResponseWriter, status int, obj interface{}) {
	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(obj)
	if err != nil {
		logrus.WithError(err).Errorf("Error encoding json response: %v", obj)
		// not using internalServerError here to avoid a potential infinite loop
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"code":500,"msg":"Error encoding json response: ` + err.Error() + `"}`))
		return
	}
	w.WriteHeader(status)
	w.Write(b)
}
