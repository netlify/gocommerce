package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
)

var genericInternalServerErrorResponse = []byte(`{"code":500,"msg":"Internal server error"}`)

// HTTPError represents an error that returns an HTTP status code.
type HTTPError interface {
	error
	Status() int
}

func badRequestError(fmtString string, args ...interface{}) HTTPError {
	return httpError(http.StatusBadRequest, fmtString, args...)
}

func internalServerError(fmtString string, args ...interface{}) HTTPError {
	return httpError(http.StatusInternalServerError, fmtString, args...)
}

func notFoundError(fmtString string, args ...interface{}) HTTPError {
	return httpError(http.StatusNotFound, fmtString, args...)
}

func unauthorizedError(fmtString string, args ...interface{}) HTTPError {
	return httpError(http.StatusUnauthorized, fmtString, args...)
}

// StatusError is an error with a message and an HTTP status code.
type StatusError struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
}

func (e StatusError) Error() string {
	return fmt.Sprintf("%d: %s", e.Code, e.Message)
}

// Status returns the HTTP status code.
func (e StatusError) Status() int {
	return e.Code
}

func httpError(code int, fmtString string, args ...interface{}) HTTPError {
	return &StatusError{
		Code:    code,
		Message: fmt.Sprintf(fmtString, args...),
	}
}

// Recoverer is a middleware that recovers from panics, logs the panic (and a
// backtrace), and returns a HTTP 500 (Internal Server Error) status if
// possible. Recoverer prints a request ID if one is provided.
func recoverer(w http.ResponseWriter, r *http.Request) (context.Context, error) {
	defer func() {
		if rvr := recover(); rvr != nil {

			logEntry := getLogEntry(r)
			if logEntry != nil {
				logEntry.Panic(rvr, debug.Stack())
			} else {
				fmt.Fprintf(os.Stderr, "Panic: %+v\n", rvr)
				debug.PrintStack()
			}

			se := StatusError{http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)}
			handleError(se, w, r)
		}
	}()

	return nil, nil
}

func handleError(err error, w http.ResponseWriter, r *http.Request) {
	switch e := err.(type) {
	case HTTPError:
		if e.Status() >= http.StatusInternalServerError {
			// this will get us the stack trace too
			getLogEntry(r).WithError(e).Error(e.Error())
		} else {
			getLogEntry(r).Warn(e.Error())
		}
		if jsonErr := sendJSON(w, e.Status(), e); jsonErr != nil {
			handleError(jsonErr, w, r)
		}
	default:
		getLogEntry(r).WithError(e).Errorf("Unhandled server error: %s", e.Error())
		// hide real error details from response to prevent info leaks
		w.WriteHeader(http.StatusInternalServerError)
		if _, writeErr := w.Write(genericInternalServerErrorResponse); writeErr != nil {
			getLogEntry(r).WithError(writeErr).Error("Error writing generic error message")
		}
	}
}
