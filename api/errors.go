package api

import (
	"fmt"
	"net/http"
	"sync"
)

func badRequestError(w http.ResponseWriter, fmtString string, args ...interface{}) *HTTPError {
	err := httpError(400, fmtString, args...)
	sendJSON(w, err.Code, err)
	return err
}

func unprocessableEntity(w http.ResponseWriter, fmtString string, args ...interface{}) *HTTPError {
	err := httpError(422, fmtString, args...)
	sendJSON(w, err.Code, err)
	return err
}

func internalServerError(w http.ResponseWriter, fmtString string, args ...interface{}) *HTTPError {
	err := httpError(500, fmtString, args...)
	sendJSON(w, err.Code, err)
	return err
}

func notFoundError(w http.ResponseWriter, fmtString string, args ...interface{}) *HTTPError {
	err := httpError(404, fmtString, args...)
	sendJSON(w, err.Code, err)
	return err
}

func unauthorizedError(w http.ResponseWriter, fmtString string, args ...interface{}) *HTTPError {
	err := httpError(401, fmtString, args...)
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

type sharedError struct {
	err   error
	mutex sync.Mutex
}

func (e *sharedError) setError(err error) {
	e.mutex.Lock()
	e.err = err
	e.mutex.Unlock()
}

func (e *sharedError) hasError() bool {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return e.err != nil
}
