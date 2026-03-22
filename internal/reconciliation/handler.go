package reconciliation

import (
	"github.com/GordenArcher/ledger-core/pkg/response"
	"github.com/gin-gonic/gin"
)

// Handler exposes the reconciliation endpoint.
type Handler struct {
	service *Service
}

// NewHandler creates a new reconciliation Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes attaches the reconciliation endpoint to the router group.
//
// Routes:
//
//	GET /reconciliation → RunReconciliation
func RegisterRoutes(rg *gin.RouterGroup, service *Service) {
	h := NewHandler(service)
	rg.GET("/reconciliation", h.RunReconciliation)
}

// RunReconciliation handles GET /reconciliation
//
// Runs a full ledger reconciliation and returns a report showing:
//   - Whether total credits - total debits == total balance (the core invariant)
//   - Any accounts where the stored balance doesn't match the ledger-computed balance
//
// A healthy system should always return balanced: true and discrepancy_count: 0.
// If not, something has gone wrong with the balance update logic.
func (h *Handler) RunReconciliation(c *gin.Context) {
	report, err := h.service.Run()
	if err != nil {
		response.InternalServerError(c, "Reconciliation failed", &response.ErrorOptions{
			Code: response.Ptr("RECONCILIATION_ERROR"),
		})
		return
	}

	// Use a different message depending on whether the ledger is balanced
	message := "Ledger is balanced"
	if !report.Balanced || report.DiscrepancyCount > 0 {
		message = "Ledger discrepancies detected"
	}

	response.OK(c, message, report, &response.SuccessOptions{
		Code: response.Ptr("RECONCILIATION_COMPLETE"),
	})
}
