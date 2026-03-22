package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIResponse is the standard envelope for all API responses.
// Every endpoint returns this shape so clients have a consistent contract.
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"` // omitted on error responses
}

// OK sends a 200 response with data and an optional message.
func OK(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// Created sends a 201 response after a successful resource creation.
func Created(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// BadRequest sends a 400 response for invalid input from the client.
func BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, APIResponse{
		Success: false,
		Message: message,
	})
}

// NotFound sends a 404 response when a requested resource does not exist.
func NotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, APIResponse{
		Success: false,
		Message: message,
	})
}

// Conflict sends a 409 response for duplicate or conflicting operations
// (e.g. duplicate idempotency key with different payload).
func Conflict(c *gin.Context, message string) {
	c.JSON(http.StatusConflict, APIResponse{
		Success: false,
		Message: message,
	})
}

// UnprocessableEntity sends a 422 response for business logic violations
// (e.g. insufficient balance, transfer to same account).
func UnprocessableEntity(c *gin.Context, message string) {
	c.JSON(http.StatusUnprocessableEntity, APIResponse{
		Success: false,
		Message: message,
	})
}

// InternalServerError sends a 500 response for unexpected server-side failures.
func InternalServerError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, APIResponse{
		Success: false,
		Message: message,
	})
}
