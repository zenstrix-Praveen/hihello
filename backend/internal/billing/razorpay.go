// Package billing preserves legacy entry points without maintaining a second payment flow.
package billing

import (
	"digital-card-platform/backend/internal/payments"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
)

type Handler struct{ DB *pgxpool.Pool }

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	(&payments.Handler{DB: h.DB}).CreateOrder(w, r)
}
func (h *Handler) VerifyPayment(w http.ResponseWriter, r *http.Request) {
	(&payments.Handler{DB: h.DB}).VerifyCheckout(w, r)
}
