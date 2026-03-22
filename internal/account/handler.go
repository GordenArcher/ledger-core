package account

import (
	"errors"

	"github.com/GordenArcher/ledger-core/pkg/response"
	"github.com/gin-gonic/gin"
)

// Handler holds the account service and exposes HTTP handler methods.
// Each method maps to one API endpoint.
type Handler struct {
	service *Service
}

// NewHandler creates a new account Handler with the given service.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes attaches account endpoints to the given Gin router group.
//
// Routes:
//
//	POST /accounts        → CreateAccount
//	GET  /accounts/:id    → GetAccount
func RegisterRoutes(rg *gin.RouterGroup, service *Service) {
	h := NewHandler(service)

	accounts := rg.Group("/accounts")
	{
		accounts.POST("", h.CreateAccount)
		accounts.GET("/:id", h.GetAccount)
	}
}

// createAccountRequest is the expected JSON body for account creation.
type createAccountRequest struct {
	OwnerName string `json:"owner_name" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	// Currency defaults to GHS if not provided
	Currency string `json:"currency"`
}

// accountResponse is the shape returned to the client for account data.
// We intentionally control what fields are exposed — the internal model
// may have fields we don't want to leak (e.g. raw balance in minor units only).
type accountResponse struct {
	ID               string  `json:"id"`
	OwnerName        string  `json:"owner_name"`
	Email            string  `json:"email"`
	Balance          int64   `json:"balance"`           // in minor units (pesewas/cents)
	BalanceFormatted float64 `json:"balance_formatted"` // in major units for display
	Currency         string  `json:"currency"`
	Status           string  `json:"status"`
	CreatedAt        string  `json:"created_at"`
}

// toAccountResponse converts an Account model to the API response shape.
func toAccountResponse(a *Account) accountResponse {
	return accountResponse{
		ID:               a.ID,
		OwnerName:        a.OwnerName,
		Email:            a.Email,
		Balance:          a.Balance,
		BalanceFormatted: a.BalanceInMajorUnit(),
		Currency:         string(a.Currency),
		Status:           string(a.Status),
		CreatedAt:        a.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// CreateAccount handles POST /accounts
//
// Creates a new ledger account with zero balance.
// Returns 409 if an account with the same email already exists.
func (h *Handler) CreateAccount(c *gin.Context) {
	var req createAccountRequest

	// Bind and validate the request body
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body", &response.ErrorOptions{
			Errors: err.Error(),
			Code:   response.Ptr("VALIDATION_ERROR"),
		})
		return
	}

	// Default currency to GHS if not provided
	currency := AccountCurrency(req.Currency)
	if currency == "" {
		currency = CurrencyGHS
	}

	account, err := h.service.CreateAccount(CreateAccountInput{
		OwnerName: req.OwnerName,
		Email:     req.Email,
		Currency:  currency,
	})

	if err != nil {
		// Map service errors to appropriate HTTP responses
		switch {
		case errors.Is(err, ErrEmailAlreadyExists):
			response.Conflict(c, err.Error(), &response.ErrorOptions{
				Code: response.Ptr("EMAIL_CONFLICT"),
			})
		case errors.Is(err, ErrInvalidCurrency):
			response.BadRequest(c, err.Error(), &response.ErrorOptions{
				Code: response.Ptr("INVALID_CURRENCY"),
			})
		default:
			response.InternalServerError(c, "Failed to create account", &response.ErrorOptions{
				Code: response.Ptr("INTERNAL_ERROR"),
			})
		}
		return
	}

	response.Created(c, "Account created successfully", toAccountResponse(account), &response.SuccessOptions{
		Code: response.Ptr("ACCOUNT_CREATED"),
	})
}

// GetAccount handles GET /accounts/:id
//
// Retrieves an account by UUID, including its current balance.
// Returns 404 if no account exists with the given ID.
func (h *Handler) GetAccount(c *gin.Context) {
	id := c.Param("id")

	if id == "" {
		response.BadRequest(c, "Account ID is required", &response.ErrorOptions{
			Code: response.Ptr("MISSING_ACCOUNT_ID"),
		})
		return
	}

	account, err := h.service.GetAccount(id)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			response.NotFound(c, "Account not found", &response.ErrorOptions{
				Code: response.Ptr("ACCOUNT_NOT_FOUND"),
			})
			return
		}
		response.InternalServerError(c, "Failed to retrieve account", &response.ErrorOptions{
			Code: response.Ptr("INTERNAL_ERROR"),
		})
		return
	}

	response.OK(c, "Account retrieved successfully", toAccountResponse(account), &response.SuccessOptions{
		Code: response.Ptr("ACCOUNT_FETCHED"),
	})
}

// ensure Handler implements http.Handler interface check at compile time
var _ interface{ CreateAccount(*gin.Context) } = (*Handler)(nil)
