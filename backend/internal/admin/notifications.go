package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	appmiddleware "digital-card-platform/backend/internal/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationHandler struct{ DB *pgxpool.Pool }

func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if n := r.URL.Query().Get("limit"); n != "" {
		if v, e := strconv.Atoi(n); e == nil && v > 0 {
			limit = v
		}
	}
	if limit > 200 {
		limit = 200
	}
	rows, e := h.DB.Query(r.Context(), `SELECT n.id,n.recipient_user_id,u.name,u.email,n.title,n.message,n.channel,n.delivery_status,n.created_at,COALESCE(a.name,'') FROM notifications n JOIN users u ON u.id=n.recipient_user_id LEFT JOIN admin_users a ON a.id=n.created_by_admin_id ORDER BY n.created_at DESC,n.id DESC LIMIT $1`, limit)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load notifications"})
		return
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var id, userID int64
		var name, email, title, message, channel, status, createdBy string
		var created interface{}
		if rows.Scan(&id, &userID, &name, &email, &title, &message, &channel, &status, &created, &createdBy) != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read notifications"})
			return
		}
		out = append(out, map[string]interface{}{"id": id, "recipient_user_id": userID, "recipient_name": name, "recipient_email": email, "title": title, "message": message, "channel": channel, "delivery_status": status, "created_at": created, "created_by": createdBy})
	}
	if rows.Err() != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not read notifications"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": out})
}

func (h *NotificationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title           string `json:"title"`
		Message         string `json:"message"`
		RecipientUserID *int64 `json:"recipient_user_id"`
		AllActive       bool   `json:"all_active"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid notification"})
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Message = strings.TrimSpace(in.Message)
	if in.Title == "" || len(in.Title) > 180 || in.Message == "" || len(in.Message) > 10000 || (!in.AllActive && in.RecipientUserID == nil) || (in.AllActive && in.RecipientUserID != nil) {
		writeJSON(w, 400, map[string]string{"error": "Enter a title and message and choose all active users or one recipient"})
		return
	}
	actor, _ := r.Context().Value(appmiddleware.AdminIDKey).(int64)
	tx, e := h.DB.Begin(r.Context())
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not create notification"})
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	var inserted int64
	if in.AllActive {
		tag, ex := tx.Exec(r.Context(), `INSERT INTO notifications(recipient_user_id,title,message,created_by_admin_id) SELECT id,$1,$2,$3 FROM users WHERE status='active'`, in.Title, in.Message, actor)
		e = ex
		inserted = tag.RowsAffected()
	} else {
		tag, ex := tx.Exec(r.Context(), `INSERT INTO notifications(recipient_user_id,title,message,created_by_admin_id) SELECT id,$1,$2,$3 FROM users WHERE id=$4 AND status='active'`, in.Title, in.Message, actor, *in.RecipientUserID)
		e = ex
		inserted = tag.RowsAffected()
	}
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not create notification"})
		return
	}
	if inserted == 0 {
		writeJSON(w, 404, map[string]string{"error": "No active recipients found"})
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not create notification"})
		return
	}
	auditAdmin(r, h.DB, "notification_created", "notification", nil, "Stored in-app notification for "+strconv.FormatInt(inserted, 10)+" recipient(s)")
	writeJSON(w, 201, map[string]interface{}{"data": map[string]interface{}{"created": inserted, "channel": "in_app", "delivery_status": "stored"}})
}
