package admin

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

func (h *ResourceHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	deletePlatformUser(w, r, h.DB)
}

// Both Admin user screens share the established soft-delete behavior. Customer
// and administrator IDs belong to separate tables and must not be compared.
func deletePlatformUser(w http.ResponseWriter, r *http.Request, db resourceDB) {
	fail := func(status int, message string) {
		writeJSON(w, status, map[string]any{"success": false, "message": message, "error": message})
	}
	id, err := adminRouteID(r)
	if err != nil {
		fail(400, "Invalid user id")
		return
	}
	actor := adminActor(r)
	if actor == nil {
		fail(401, "Administrator authentication is required")
		return
	}
	tx, err := db.Begin(r.Context())
	if err != nil {
		fail(500, "Could not delete user.")
		return
	}
	defer tx.Rollback(r.Context())
	var email string
	var deletedAt *time.Time
	err = tx.QueryRow(r.Context(), `SELECT email,deleted_at FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&email, &deletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(404, "User not found")
		return
	}
	if err != nil {
		fail(500, "Could not delete user.")
		return
	}
	if deletedAt != nil {
		fail(409, "User has already been deleted")
		return
	}
	var self bool
	if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM admin_users WHERE id=$1 AND LOWER(email)=LOWER($2))`, *actor, email).Scan(&self); err != nil {
		fail(500, "Could not delete user.")
		return
	}
	if self {
		fail(409, "You cannot delete the user matching your currently authenticated Admin account.")
		return
	}
	for _, sql := range []string{
		`UPDATE digital_cards SET status='archived',updated_at=NOW() WHERE user_id=$1 AND status<>'archived'`,
		`UPDATE subscriptions SET status='cancelled',updated_at=NOW() WHERE user_id=$1 AND status IN ('active','trial','past_due')`,
		`UPDATE auth_otp_codes SET used_at=COALESCE(used_at,NOW()),verification_token_used_at=COALESCE(verification_token_used_at,NOW()) WHERE subject_type='customer' AND subject_id=$1`,
	} {
		if _, err = tx.Exec(r.Context(), sql, id); err != nil {
			fail(500, "Could not delete user.")
			return
		}
	}
	if _, err = tx.Exec(r.Context(), `UPDATE users SET name='Deleted User',email=$1,status='inactive',password_hash=NULL,deleted_at=NOW(),updated_at=NOW() WHERE id=$2`, fmt.Sprintf("deleted+%d@deleted.invalid", id), id); err != nil {
		fail(500, "Could not delete user.")
		return
	}
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO admin_security_activity(admin_user_id,event_type,action,entity_type,entity_id,description,ip_address,user_agent,metadata) VALUES($1,'administration_user_deleted','administration_user_deleted','user',$2,'Removed customer access and archived public cards; retained billing history',NULLIF($3,'')::inet,$4,'{}'::jsonb)`, *actor, fmt.Sprint(id), ip, r.UserAgent()); err != nil {
		fail(500, "Could not delete user.")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(500, "Could not delete user.")
		return
	}
	writeJSON(w, 200, map[string]any{"success": true, "message": "User deleted successfully.", "data": nil})
}
