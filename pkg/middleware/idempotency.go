package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/GordenArcher/ledger-core/internal/idempotency"
	"github.com/gin-gonic/gin"
)

// responseWriter wraps gin.ResponseWriter so we can capture the response
// body and status code after the handler writes them.
type responseWriter struct {
	gin.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

// Write intercepts the response body as it's being written.
func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

// WriteHeader intercepts the status code as it's being written.
func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Idempotency returns a Gin middleware that deduplicates write operations
// using the Idempotency-Key request header.
//
// Behavior:
//   - If no Idempotency-Key header is present, the request passes through normally.
//   - If the key is seen for the first time, the request is processed and the
//     response is cached for 24 hours.
//   - If the key has been seen before on the same endpoint, the cached response
//     is returned immediately without hitting the handler.
//   - If the key was used on a different endpoint, a 422 is returned.
//
// Applied only to POST routes via router group middleware.
func Idempotency(repo *idempotency.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Idempotency-Key")

		// No key provided, pass through without caching
		if key == "" {
			c.Next()
			return
		}

		// Look up the key in the DB
		existing, err := repo.Find(key)
		if err != nil && !idempotency.IsNotFound(err) {
			// DB error, fail open (let the request through) rather than blocking
			c.Next()
			return
		}

		if existing != nil {
			// Key seen before, validate it's for the same endpoint
			if existing.RequestPath != c.FullPath() {
				c.JSON(http.StatusUnprocessableEntity, gin.H{
					"status":      "error",
					"message":     "Idempotency key was used on a different endpoint",
					"http_status": http.StatusUnprocessableEntity,
					"code":        "IDEMPOTENCY_KEY_MISMATCH",
				})
				c.Abort()
				return
			}

			// Return the cached response, same status code and body
			c.Header("X-Idempotent-Replayed", "true")
			c.Data(existing.StatusCode, "application/json", []byte(existing.ResponseBody))
			c.Abort()
			return
		}

		// First time seeing this key, wrap the response writer to capture output
		wrapped := &responseWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
			statusCode:     http.StatusOK,
		}
		c.Writer = wrapped

		// Process the request normally
		c.Next()

		// After the handler runs, cache successful responses (2xx) only.
		// Don't cache validation errors, clients should fix and retry those.
		if wrapped.statusCode >= 200 && wrapped.statusCode < 300 {
			if json.Valid(wrapped.body.Bytes()) {
				record := &idempotency.Record{
					Key:          key,
					StatusCode:   wrapped.statusCode,
					ResponseBody: wrapped.body.String(),
					RequestPath:  c.FullPath(),
					ExpiresAt:    time.Now().Add(24 * time.Hour),
				}
				// Best-effort save, don't fail the request if caching fails
				_ = repo.Save(record)
			}
		}
	}
}

// compile-time check that responseWriter satisfies io.Writer
var _ io.Writer = (*responseWriter)(nil)
