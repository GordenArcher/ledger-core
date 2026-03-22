// internal/transfer/handler.go

package transfer

import (
	"errors"
	"net/http"

	"github.com/GordenArcher/ledger-core/pkg/response"
	"github.com/gin-gonic/gin"
)

// Handler exposes HTTP endpoints for transfer operations.
type Handler struct {
	service *Service
}

// NewHandler creates a new transfer Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes attaches transfer endpoints to the given Gin router group.
//
// Routes:
//
//	POST /transfers                        → ExecuteTransfer
//	GET  /transfers/:id                    → GetTransfer
//	GET  /accounts/:id/transfers           → GetTransfersByAccount
func RegisterRoutes(rg *gin.RouterGroup, service *Service) {
	h := NewHandler(service)

	// Transfer CRUD
	transfers := rg.Group("/transfers")
	{
		transfers.POST("", h.ExecuteTransfer)
		transfers.GET("/:id", h.GetTransfer)
	}

	// Account-scoped transfer history
	rg.GET("/accounts/:id/transfers", h.GetTransfersByAccount)
}

// transferRequest is the expected JSON body for initiating a transfer.
type transferRequest struct {
	FromAccountID  string `json:"from_account_id" binding:"required"`
	ToAccountID    string `json:"to_account_id" binding:"required"`
	Amount         int64  `json:"amount" binding:"required,gt=0"`
	Reference      string `json:"reference"`       // optional
	IdempotencyKey string `json:"idempotency_key"` // optional here, enforced in Phase 6
}

// transferResponse is the shape returned to the client after a transfer.
type transferResponse struct {
	ID              string  `json:"id"`
	FromAccountID   string  `json:"from_account_id"`
	ToAccountID     string  `json:"to_account_id"`
	Amount          int64   `json:"amount"`
	AmountFormatted float64 `json:"amount_formatted"`
	Currency        string  `json:"currency"`
	Reference       string  `json:"reference"`
	Status          string  `json:"status"`
	CreatedAt       string  `json:"created_at"`
}

// toTransferResponse converts a Transfer model to the API response shape.
func toTransferResponse(t *Transfer) transferResponse {
	return transferResponse{
		ID:              t.ID,
		FromAccountID:   t.FromAccountID,
		ToAccountID:     t.ToAccountID,
		Amount:          t.Amount,
		AmountFormatted: float64(t.Amount) / 100,
		Currency:        t.Currency,
		Reference:       t.Reference,
		Status:          string(t.Status),
		CreatedAt:       t.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// ExecuteTransfer handles POST /transfers
//
// Atomically debits the sender and credits the receiver.
// Both balance changes happen in a single DB transaction — if either fails,
// both roll back and no money moves.
func (h *Handler) ExecuteTransfer(c *gin.Context) {
	var req transferRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body", &response.ErrorOptions{
			Errors: err.Error(),
			Code:   response.Ptr("VALIDATION_ERROR"),
		})
		return
	}

	transfer, err := h.service.Execute(TransferInput{
		FromAccountID:  req.FromAccountID,
		ToAccountID:    req.ToAccountID,
		Amount:         req.Amount,
		Reference:      req.Reference,
		IdempotencyKey: req.IdempotencyKey,
	})

	if err != nil {
		switch {
		case errors.Is(err, ErrSameAccount):
			response.BadRequest(c, err.Error(), &response.ErrorOptions{
				Code: response.Ptr("SAME_ACCOUNT"),
			})
		case errors.Is(err, ErrInvalidAmount):
			response.BadRequest(c, err.Error(), &response.ErrorOptions{
				Code: response.Ptr("INVALID_AMOUNT"),
			})
		case errors.Is(err, ErrInsufficientBalance):
			response.UnprocessableEntity(c, err.Error(), &response.ErrorOptions{
				Code: response.Ptr("INSUFFICIENT_BALANCE"),
			})
		case errors.Is(err, ErrCurrencyMismatch):
			response.UnprocessableEntity(c, err.Error(), &response.ErrorOptions{
				Code: response.Ptr("CURRENCY_MISMATCH"),
			})
		case errors.Is(err, ErrDuplicateTransfer):
			response.Conflict(c, err.Error(), &response.ErrorOptions{
				Code: response.Ptr("DUPLICATE_TRANSFER"),
			})
		default:
			response.InternalServerError(c, "Transfer failed", &response.ErrorOptions{
				Errors: err.Error(),
				Code:   response.Ptr("INTERNAL_ERROR"),
			})
		}
		return
	}

	response.Created(c, "Transfer completed successfully", toTransferResponse(transfer), &response.SuccessOptions{
		Code: response.Ptr("TRANSFER_COMPLETED"),
	})
}

// GetTransfer handles GET /transfers/:id
//
// Retrieves a single transfer record by UUID.
func (h *Handler) GetTransfer(c *gin.Context) {
	id := c.Param("id")

	transfer, err := h.service.GetTransfer(id)
	if err != nil {
		if errors.Is(err, ErrTransferNotFound) {
			response.NotFound(c, "Transfer not found", &response.ErrorOptions{
				Code: response.Ptr("TRANSFER_NOT_FOUND"),
			})
			return
		}
		response.InternalServerError(c, "Failed to retrieve transfer", &response.ErrorOptions{
			Code: response.Ptr("INTERNAL_ERROR"),
		})
		return
	}

	response.OK(c, "Transfer retrieved successfully", toTransferResponse(transfer), &response.SuccessOptions{
		Code: response.Ptr("TRANSFER_FETCHED"),
	})
}

// GetTransfersByAccount handles GET /accounts/:id/transfers
//
// Returns all transfers where the given account is either sender or receiver,
// ordered by most recent first.
func (h *Handler) GetTransfersByAccount(c *gin.Context) {
	accountID := c.Param("id")

	transfers, err := h.service.GetTransfersByAccount(accountID)
	if err != nil {
		response.InternalServerError(c, "Failed to retrieve transfers", &response.ErrorOptions{
			Code: response.Ptr("INTERNAL_ERROR"),
		})
		return
	}

	result := make([]transferResponse, len(transfers))
	for i, t := range transfers {
		result[i] = toTransferResponse(&t)
	}

	response.OK(c, "Transfers retrieved successfully", result, &response.SuccessOptions{
		Code: response.Ptr("TRANSFERS_FETCHED"),
	})
}

// compile-time check
var _ http.Handler = (*gin.Engine)(nil)
