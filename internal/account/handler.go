// internal/account/handler.go

package account

import (
	"errors"
	"net/http"

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
//	POST /accounts              → CreateAccount
//	GET  /accounts/:id          → GetAccount
//	POST /accounts/:id/deposit  → Deposit
//	POST /accounts/:id/withdraw → Withdraw
func RegisterRoutes(rg *gin.RouterGroup, service *Service) {
	h := NewHandler(service)

	accounts := rg.Group("/accounts")
	{
		accounts.POST("", h.CreateAccount)

		// Single account operations grouped under /:id
		account := accounts.Group("/:id")
		{
			account.GET("", h.GetAccount)
			account.POST("/deposit", h.Deposit)
			account.POST("/withdraw", h.Withdraw)
		}
	}
}

// createAccountRequest is the expected JSON body for account creation.
type createAccountRequest struct {
	OwnerName string `json:"owner_name" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	// Currency defaults to GHS if not provided
	Currency string `json:"currency"`
}

// amountRequest is the expected JSON body for deposit and withdraw operations.
// Amount must be in minor units (pesewas/cents). e.g. GHS 10.50 → 1050
type amountRequest struct {
	Amount int64 `json:"amount" binding:"required,gt=0"`
}

// accountResponse is the shape returned to the client for account data.
// We intentionally control what fields are exposed — the internal model
// may have fields we don't want to leak.
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

// Deposit handles POST /accounts/:id/deposit
//
// Credits the given amount (in minor units) to the account's balance.
// Uses a SELECT FOR UPDATE transaction internally to prevent race conditions.
//
// Example body: {"amount": 1050} → deposits GHS 10.50
func (h *Handler) Deposit(c *gin.Context) {
	id := c.Param("id")

	var req amountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body", &response.ErrorOptions{
			Errors: err.Error(),
			Code:   response.Ptr("VALIDATION_ERROR"),
		})
		return
	}

	account, err := h.service.Deposit(DepositInput{
		AccountID: id,
		Amount:    req.Amount,
	})

	if err != nil {
		switch {
		case errors.Is(err, ErrAccountNotFound):
			response.NotFound(c, "Account not found", &response.ErrorOptions{
				Code: response.Ptr("ACCOUNT_NOT_FOUND"),
			})
		case errors.Is(err, ErrAccountNotActive):
			response.UnprocessableEntity(c, err.Error(), &response.ErrorOptions{
				Code: response.Ptr("ACCOUNT_NOT_ACTIVE"),
			})
		case errors.Is(err, ErrInvalidAmount):
			response.BadRequest(c, err.Error(), &response.ErrorOptions{
				Code: response.Ptr("INVALID_AMOUNT"),
			})
		default:
			response.InternalServerError(c, "Deposit failed", &response.ErrorOptions{
				Code: response.Ptr("INTERNAL_ERROR"),
			})
		}
		return
	}

	response.OK(c, "Deposit successful", toAccountResponse(account), &response.SuccessOptions{
		Code: response.Ptr("DEPOSIT_SUCCESS"),
	})
}

// Withdraw handles POST /accounts/:id/withdraw
//
// Debits the given amount (in minor units) from the account's balance.
// Returns 422 if the account has insufficient funds.
//
// Example body: {"amount": 500} → withdraws GHS 5.00
func (h *Handler) Withdraw(c *gin.Context) {
	id := c.Param("id")

	var req amountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body", &response.ErrorOptions{
			Errors: err.Error(),
			Code:   response.Ptr("VALIDATION_ERROR"),
		})
		return
	}

	account, err := h.service.Withdraw(WithdrawInput{
		AccountID: id,
		Amount:    req.Amount,
	})

	if err != nil {
		switch {
		case errors.Is(err, ErrAccountNotFound):
			response.NotFound(c, "Account not found", &response.ErrorOptions{
				Code: response.Ptr("ACCOUNT_NOT_FOUND"),
			})
		case errors.Is(err, ErrAccountNotActive):
			response.UnprocessableEntity(c, err.Error(), &response.ErrorOptions{
				Code: response.Ptr("ACCOUNT_NOT_ACTIVE"),
			})
		case errors.Is(err, ErrInsufficientBalance):
			response.UnprocessableEntity(c, err.Error(), &response.ErrorOptions{
				Code: response.Ptr("INSUFFICIENT_BALANCE"),
			})
		case errors.Is(err, ErrInvalidAmount):
			response.BadRequest(c, err.Error(), &response.ErrorOptions{
				Code: response.Ptr("INVALID_AMOUNT"),
			})
		default:
			response.InternalServerError(c, "Withdrawal failed", &response.ErrorOptions{
				Code: response.Ptr("INTERNAL_ERROR"),
			})
		}
		return
	}

	response.OK(c, "Withdrawal successful", toAccountResponse(account), &response.SuccessOptions{
		Code: response.Ptr("WITHDRAWAL_SUCCESS"),
	})
}

// compile-time check
var _ http.Handler = (*gin.Engine)(nil)
