package api

import (
	"net/http"
	"sync"
)

// BadRequestError is simple Error Wrapper
func BadRequestError(w http.ResponseWriter, message string) *HTTPError {
	err := &HTTPError{Code: 400, Message: message}
	sendJSON(w, err.Code, err)
	return err
}

// UnprocessableEntity is simple Error Wrapper
func UnprocessableEntity(w http.ResponseWriter, message string) *HTTPError {
	err := &HTTPError{Code: 422, Message: message}
	sendJSON(w, err.Code, err)
	return err
}

// InternalServerError is simple Error Wrapper
func InternalServerError(w http.ResponseWriter, message string) *HTTPError {
	err := &HTTPError{Code: 500, Message: message}
	sendJSON(w, err.Code, err)
	return err
}

// NotFoundError is simple Error Wrapper
func NotFoundError(w http.ResponseWriter, message string) *HTTPError {
	err := &HTTPError{Code: 404, Message: message}
	sendJSON(w, err.Code, err)
	return err
}

// UnauthorizedError is simple Error Wrapper
func UnauthorizedError(w http.ResponseWriter, message string) *HTTPError {
	err := &HTTPError{Code: 401, Message: message}
	sendJSON(w, err.Code, err)
	return err
}

type sharedError struct {
	err   error
	mutex sync.Mutex
}

// SetError is a threadsafe setter
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
