package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const organizationDeleteConflict = "This organization cannot be deleted because users, cards, subscriptions, payments, QR codes, or NFC tags are associated with it."

func organizationDeleteError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"success": false, "message": message, "error": message})
}

func (h *ResourceHandler) DeleteOrganization(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		organizationDeleteError(w, 400, "Invalid organization id")
		return
	}
	fail := func(err error) {
		var pgerr *pgconn.PgError
		if errors.As(err, &pgerr) && pgerr.Code == "23503" {
			organizationDeleteError(w, 409, organizationDeleteConflict)
			return
		}
		organizationDeleteError(w, 500, "Could not delete organization.")
	}
	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		fail(err)
		return
	}
	defer tx.Rollback(r.Context())
	// Serialize deletion against other deletes and new foreign-key references.
	err = tx.QueryRow(r.Context(), `SELECT id FROM organizations WHERE id=$1 FOR UPDATE`, id).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		organizationDeleteError(w, 404, "Organization not found")
		return
	}
	if err != nil {
		fail(err)
		return
	}
	var referenced bool
	// Users belong to organizations through digital_cards, not a users column.
	err = tx.QueryRow(r.Context(), `SELECT
 EXISTS(SELECT 1 FROM digital_cards WHERE organization_id=$1) OR
 EXISTS(SELECT 1 FROM subscriptions WHERE organization_id=$1) OR
 EXISTS(SELECT 1 FROM payments WHERE organization_id=$1) OR
 EXISTS(SELECT 1 FROM qr_codes WHERE organization_id=$1) OR
 EXISTS(SELECT 1 FROM nfc_tags WHERE organization_id=$1)`, id).Scan(&referenced)
	if err != nil {
		fail(err)
		return
	}
	if referenced {
		organizationDeleteError(w, 409, organizationDeleteConflict)
		return
	}
	if _, err = tx.Exec(r.Context(), `DELETE FROM organizations WHERE id=$1`, id); err != nil {
		fail(err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(err)
		return
	}
	writeJSON(w, 200, map[string]any{"success": true, "message": "Organization deleted successfully.", "data": nil})
}
