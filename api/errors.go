package api

import (
	"fmt"
	"net/http"
)

func badRequestError(w http.ResponseWriter, fmtString string, args ...interface{}) *HTTPError {
	err := httpError(http.StatusBadRequest, fmtString, args...)
	sendJSON(w, err.Code, err)
	return err
}

func internalServerError(w http.ResponseWriter, fmtString string, args ...interface{}) *HTTPError {
	err := httpError(http.StatusInternalServerError, fmtString, args...)
	sendJSON(w, err.Code, err)
	return err
}

func notFoundError(w http.ResponseWriter, fmtString string, args ...interface{}) *HTTPError {
	err := httpError(http.StatusNotFound, fmtString, args...)
	sendJSON(w, err.Code, err)
	return err
}

func unauthorizedError(w http.ResponseWriter, fmtString string, args ...interface{}) *HTTPError {
	err := httpError(http.StatusUnauthorized, fmtString, args...)
	sendJSON(w, err.Code, err)
	return err
}

// HTTPError is an error with a message
type HTTPError struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("%d: %s", e.Code, e.Message)
}

func httpError(code int, fmtString string, args ...interface{}) *HTTPError {
	return &HTTPError{
		Code:    code,
		Message: fmt.Sprintf(fmtString, args...),
	}
}
