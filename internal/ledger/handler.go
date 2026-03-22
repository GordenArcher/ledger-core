package ledger

import (
	"net/http"
	"strconv"

	"github.com/GordenArcher/ledger-core/pkg/response"
	"github.com/gin-gonic/gin"
)

// Handler exposes HTTP endpoints for ledger queries.
type Handler struct {
	service *Service
}

// NewHandler creates a new ledger Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes attaches ledger endpoints to the given Gin router group.
//
// Routes:
//
//	GET /accounts/:id/ledger → GetAccountLedger (paginated)
func RegisterRoutes(rg *gin.RouterGroup, service *Service) {
	h := NewHandler(service)
	rg.GET("/accounts/:id/ledger", h.GetAccountLedger)
}

// entryResponse is the shape returned to the client for a ledger entry.
type entryResponse struct {
	ID                    string  `json:"id"`
	AccountID             string  `json:"account_id"`
	Type                  string  `json:"type"`
	Source                string  `json:"source"`
	Amount                int64   `json:"amount"`
	AmountFormatted       float64 `json:"amount_formatted"`
	BalanceAfter          int64   `json:"balance_after"`
	BalanceAfterFormatted float64 `json:"balance_after_formatted"`
	TransferID            *string `json:"transfer_id,omitempty"`
	Description           string  `json:"description"`
	CreatedAt             string  `json:"created_at"`
}

// toEntryResponse converts a ledger Entry to the API response shape.
func toEntryResponse(e Entry) entryResponse {
	return entryResponse{
		ID:                    e.ID,
		AccountID:             e.AccountID,
		Type:                  string(e.Type),
		Source:                string(e.Source),
		Amount:                e.Amount,
		AmountFormatted:       float64(e.Amount) / 100,
		BalanceAfter:          e.BalanceAfter,
		BalanceAfterFormatted: float64(e.BalanceAfter) / 100,
		TransferID:            e.TransferID,
		Description:           e.Description,
		CreatedAt:             e.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// GetAccountLedger handles GET /accounts/:id/ledger
//
// Returns a paginated list of all ledger entries for the given account.
// Query params: page (default 1), page_size (default 20, max 100)
//
// Example: GET /api/v1/accounts/<id>/ledger?page=1&page_size=20
func (h *Handler) GetAccountLedger(c *gin.Context) {
	accountID := c.Param("id")

	// Parse pagination query params with sensible defaults
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	entries, total, err := h.service.GetAccountLedgerPaginated(accountID, page, pageSize)
	if err != nil {
		response.InternalServerError(c, "Failed to retrieve ledger", &response.ErrorOptions{
			Code: response.Ptr("INTERNAL_ERROR"),
		})
		return
	}

	// Build response list
	result := make([]entryResponse, len(entries))
	for i, e := range entries {
		result[i] = toEntryResponse(e)
	}

	// Include pagination metadata in the response
	response.OK(c, "Ledger retrieved successfully", result, &response.SuccessOptions{
		Code: response.Ptr("LEDGER_FETCHED"),
		Meta: map[string]any{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

// compile-time check
var _ http.Handler = (*gin.Engine)(nil)
