package api

import "net/http"

// BadRequestError is simple Error Wrapper
func BadRequestError(w http.ResponseWriter, message string) {
	sendJSON(w, 400, &HTTPError{Code: 400, Message: message})
}

// UnprocessableEntity is simple Error Wrapper
func UnprocessableEntity(w http.ResponseWriter, message string) {
	sendJSON(w, 422, &HTTPError{Code: 422, Message: message})
}

// InternalServerError is simple Error Wrapper
func InternalServerError(w http.ResponseWriter, message string) {
	sendJSON(w, 500, &HTTPError{Code: 500, Message: message})
}

// NotFoundError is simple Error Wrapper
func NotFoundError(w http.ResponseWriter, message string) {
	sendJSON(w, 404, &HTTPError{Code: 404, Message: message})
}

// UnauthorizedError is simple Error Wrapper
func UnauthorizedError(w http.ResponseWriter, message string) {
	sendJSON(w, 401, &HTTPError{Code: 401, Message: message})
}
