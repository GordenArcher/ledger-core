package idempotency

import "time"

// Record stores the cached response for a previously seen idempotency key.
//
// When a client retries a request with the same key, we return this
// cached response instead of re-executing the handler, ensuring the
// operation is only applied once regardless of how many times it's retried.
type Record struct {
	// Key is the client-provided idempotency key (UUID recommended)
	Key string `gorm:"type:varchar(255);primaryKey" json:"key"`

	// StatusCode is the HTTP status code of the original response
	StatusCode int `gorm:"not null" json:"status_code"`

	// ResponseBody is the full JSON response body, stored as text
	ResponseBody string `gorm:"type:text;not null" json:"response_body"`

	// RequestPath ensures a key cannot be reused across different endpoints.
	// e.g. the same key for POST /deposit and POST /transfer should be rejected.
	RequestPath string `gorm:"type:varchar(255);not null" json:"request_path"`

	// ExpiresAt defines when this record can be cleaned up.
	// Keys are valid for 24 hours, after that, the same key can be reused.
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`

	CreatedAt time.Time `json:"created_at"`
}
