package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response is the standard envelope for all API responses.
// Every endpoint returns this shape so clients have a consistent contract.
//
// Success responses use "status": "success"
// Error responses use "status": "error"
//
// Optional fields (Code, RequestID, Meta, Data, Errors) are omitted from
// the JSON output when not provided, keeping responses clean.
type Response struct {
	Status     string         `json:"status"`
	Message    string         `json:"message"`
	HTTPStatus int            `json:"http_status"`
	Data       interface{}    `json:"data,omitempty"`
	Errors     interface{}    `json:"errors,omitempty"`
	Code       *string        `json:"code,omitempty"`
	RequestID  *string        `json:"request_id,omitempty"`
	Meta       map[string]any `json:"meta,omitempty"`
}

// SuccessOptions holds optional fields for a success response.
type SuccessOptions struct {
	Code      *string
	RequestID *string
	Meta      map[string]any
}

// ErrorOptions holds optional fields for an error response.
type ErrorOptions struct {
	Errors    interface{} // can be a string, map, or list
	Code      *string
	RequestID *string
	Meta      map[string]any
}

// Success sends a structured success response.
//
// Example:
//
//	response.Success(c, http.StatusCreated, "Account created successfully", gin.H{
//	    "id": account.ID,
//	}, &response.SuccessOptions{
//	    Code: response.Ptr("ACCOUNT_CREATED"),
//	    Meta: map[string]any{"timestamp": time.Now()},
//	})
func Success(c *gin.Context, statusCode int, message string, data interface{}, opts *SuccessOptions) {
	payload := Response{
		Status:     "success",
		Message:    message,
		HTTPStatus: statusCode,
		Data:       data,
	}

	if opts != nil {
		payload.Code = opts.Code
		payload.RequestID = opts.RequestID
		payload.Meta = opts.Meta
	}

	c.JSON(statusCode, payload)
}

// Error sends a structured error response.
//
// Example:
//
//	response.Error(c, http.StatusBadRequest, "Validation failed", &response.ErrorOptions{
//	    Errors: map[string]any{"amount": "must be greater than zero"},
//	    Code:   response.Ptr("VALIDATION_ERROR"),
//	})
func Error(c *gin.Context, statusCode int, message string, opts *ErrorOptions) {
	payload := Response{
		Status:     "error",
		Message:    message,
		HTTPStatus: statusCode,
	}

	if opts != nil {
		payload.Errors = opts.Errors
		payload.Code = opts.Code
		payload.RequestID = opts.RequestID
		payload.Meta = opts.Meta
	}

	c.JSON(statusCode, payload)
}

// Convenience wrappers for common HTTP status codes

// OK sends a 200 success response.
func OK(c *gin.Context, message string, data interface{}, opts *SuccessOptions) {
	Success(c, http.StatusOK, message, data, opts)
}

// Created sends a 201 success response after resource creation.
func Created(c *gin.Context, message string, data interface{}, opts *SuccessOptions) {
	Success(c, http.StatusCreated, message, data, opts)
}

// BadRequest sends a 400 error response for invalid client input.
func BadRequest(c *gin.Context, message string, opts *ErrorOptions) {
	Error(c, http.StatusBadRequest, message, opts)
}

// NotFound sends a 404 error response when a resource does not exist.
func NotFound(c *gin.Context, message string, opts *ErrorOptions) {
	Error(c, http.StatusNotFound, message, opts)
}

// Conflict sends a 409 error response for duplicate or conflicting operations.
func Conflict(c *gin.Context, message string, opts *ErrorOptions) {
	Error(c, http.StatusConflict, message, opts)
}

// UnprocessableEntity sends a 422 error response for business logic violations
// (e.g. insufficient balance, transfer to the same account).
func UnprocessableEntity(c *gin.Context, message string, opts *ErrorOptions) {
	Error(c, http.StatusUnprocessableEntity, message, opts)
}

// InternalServerError sends a 500 error response for unexpected server failures.
func InternalServerError(c *gin.Context, message string, opts *ErrorOptions) {
	Error(c, http.StatusInternalServerError, message, opts)
}

// Ptr is a helper to convert a string literal into a *string.
// Useful for passing Code and RequestID without declaring a variable first.
//
// Example: response.Ptr("ACCOUNT_CREATED")
func Ptr(s string) *string {
	return &s
}
