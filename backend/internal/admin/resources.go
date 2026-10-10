package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ResourceHandler exposes the PostgreSQL-backed records used by the admin console.
type resourceDB interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Begin(context.Context) (pgx.Tx, error)
}

type ResourceHandler struct{ DB resourceDB }

type recordInput struct {
	Name   string `json:"name"`
	Email  string `json:"email"`
	Status string `json:"status"`
}

type contactInput struct {
	Name   string `json:"name"`
	Email  string `json:"email"`
	Phone  string `json:"phone"`
	Source string `json:"source"`
	Status string `json:"status"`
	Notes  string `json:"notes"`
	UserID *int64 `json:"user_id"`
}

var contactStatuses = map[string]bool{"new": true, "contacted": true, "qualified": true, "converted": true, "closed": true}
var contactSources = map[string]bool{"Manual": true, "Digital Card": true, "QR Scan": true, "NFC": true, "Website": true, "Referral": true, "Other": true}

func (h *ResourceHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	page, limit := pagination(r)
	search := "%" + strings.TrimSpace(r.URL.Query().Get("search")) + "%"
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	var total int64
	if err := h.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND (name ILIKE $1 OR email ILIKE $1) AND ($2='' OR status=$2)`, search, status).Scan(&total); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not count users"})
		return
	}
	rows, err := h.DB.Query(r.Context(), `SELECT id,public_id::text,name,email,status,created_at,updated_at FROM users WHERE deleted_at IS NULL AND (name ILIKE $1 OR email ILIKE $1) AND ($2='' OR status=$2) ORDER BY created_at DESC LIMIT $3 OFFSET $4`, search, status, limit, (page-1)*limit)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load users"})
		return
	}
	defer rows.Close()
	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int64
		var publicID, name, email, status string
		var createdAt, updatedAt interface{}
		if err := rows.Scan(&id, &publicID, &name, &email, &status, &createdAt, &updatedAt); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read users"})
			return
		}
		items = append(items, map[string]interface{}{"id": id, "public_id": publicID, "name": name, "email": email, "status": status, "created_at": createdAt, "updated_at": updatedAt})
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not read users"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": items, "pagination": map[string]int64{"page": int64(page), "limit": int64(limit), "total": total, "pages": (total + int64(limit) - 1) / int64(limit)}})
}

func (h *ResourceHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		writeJSON(w, 400, map[string]string{"error": "Invalid user id"})
		return
	}
	var user map[string]interface{} = make(map[string]interface{})
	var publicID, name, email, status string
	var createdAt, updatedAt interface{}
	err = h.DB.QueryRow(r.Context(), `SELECT id,public_id::text,name,email,status,created_at,updated_at FROM users WHERE id=$1 AND deleted_at IS NULL`, id).Scan(&id, &publicID, &name, &email, &status, &createdAt, &updatedAt)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "User not found"})
		return
	}
	user = map[string]interface{}{"id": id, "public_id": publicID, "name": name, "email": email, "status": status, "created_at": createdAt, "updated_at": updatedAt}
	writeJSON(w, 200, map[string]interface{}{"data": user})
}

func pagination(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}

func (h *ResourceHandler) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	page, limit := pagination(r)
	search := "%" + strings.TrimSpace(r.URL.Query().Get("search")) + "%"
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	var total int64
	if err := h.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM organizations WHERE (name ILIKE $1 OR COALESCE(email,'') ILIKE $1) AND ($2='' OR status=$2)`, search, status).Scan(&total); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not count organizations"})
		return
	}
	rows, err := h.DB.Query(r.Context(), `SELECT o.id,o.public_id::text,o.name,o.email,o.status,o.created_at,o.updated_at,COUNT(DISTINCT c.id),COUNT(DISTINCT c.user_id) FROM organizations o LEFT JOIN digital_cards c ON c.organization_id=o.id WHERE (o.name ILIKE $1 OR COALESCE(o.email,'') ILIKE $1) AND ($2='' OR o.status=$2) GROUP BY o.id ORDER BY o.created_at DESC LIMIT $3 OFFSET $4`, search, status, limit, (page-1)*limit)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load organizations"})
		return
	}
	defer rows.Close()
	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int64
		var publicID, name, status string
		var email *string
		var createdAt, updatedAt interface{}
		var cardCount, userCount int64
		if err := rows.Scan(&id, &publicID, &name, &email, &status, &createdAt, &updatedAt, &cardCount, &userCount); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read organizations"})
			return
		}
		items = append(items, map[string]interface{}{"id": id, "public_id": publicID, "name": name, "email": email, "status": status, "created_at": createdAt, "updated_at": updatedAt, "card_count": cardCount, "user_count": userCount})
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not read organizations"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": items, "pagination": map[string]int64{"page": int64(page), "limit": int64(limit), "total": total, "pages": (total + int64(limit) - 1) / int64(limit)}})
}

func (h *ResourceHandler) GetOrganization(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		writeJSON(w, 400, map[string]string{"error": "Invalid organization id"})
		return
	}
	var publicID, name, status string
	var email *string
	var createdAt interface{}
	err = h.DB.QueryRow(r.Context(), `SELECT o.public_id::text,o.name,o.email,o.status,o.created_at FROM organizations o WHERE o.id=$1`, id).Scan(&publicID, &name, &email, &status, &createdAt)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "Organization not found"})
		return
	}
	users, err := h.DB.Query(r.Context(), `SELECT DISTINCT u.id,u.name,u.email,u.status FROM users u JOIN digital_cards c ON c.user_id=u.id WHERE c.organization_id=$1 ORDER BY u.name LIMIT 100`, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load related users"})
		return
	}
	userItems := make([]map[string]interface{}, 0)
	for users.Next() {
		var uid int64
		var un, ue, us string
		if err := users.Scan(&uid, &un, &ue, &us); err != nil {
			users.Close()
			writeJSON(w, 500, map[string]string{"error": "Could not read related users"})
			return
		}
		userItems = append(userItems, map[string]interface{}{"id": uid, "name": un, "email": ue, "status": us})
	}
	users.Close()
	cards, err := h.DB.Query(r.Context(), `SELECT id,name,slug,status,created_at FROM digital_cards WHERE organization_id=$1 ORDER BY created_at DESC LIMIT 100`, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load related cards"})
		return
	}
	cardItems := make([]map[string]interface{}, 0)
	for cards.Next() {
		var cid int64
		var cn, slug, cs string
		var cc interface{}
		if err := cards.Scan(&cid, &cn, &slug, &cs, &cc); err != nil {
			cards.Close()
			writeJSON(w, 500, map[string]string{"error": "Could not read related cards"})
			return
		}
		cardItems = append(cardItems, map[string]interface{}{"id": cid, "name": cn, "slug": slug, "status": cs, "created_at": cc})
	}
	cards.Close()
	writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"id": id, "public_id": publicID, "name": name, "email": email, "status": status, "created_at": createdAt, "users": userItems, "cards": cardItems}})
}

func (h *ResourceHandler) ListContacts(w http.ResponseWriter, r *http.Request) {
	page, limit := pagination(r)
	search := "%" + strings.TrimSpace(r.URL.Query().Get("search")) + "%"
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	var total int64
	if err := h.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM contacts c WHERE (c.name ILIKE $1 OR COALESCE(c.email,'') ILIKE $1 OR COALESCE(c.phone,'') ILIKE $1 OR COALESCE(c.source,'') ILIKE $1) AND ($2='' OR c.status=$2)`, search, status).Scan(&total); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not count contacts"})
		return
	}
	rows, err := h.DB.Query(r.Context(), `SELECT c.id,c.public_id::text,c.user_id,c.name,c.email,c.phone,CASE WHEN c.source='digital_card' THEN 'Digital Card' ELSE COALESCE(c.source,'Other') END,c.status,c.notes,c.created_at,COALESCE(u.name,''),COALESCE(u.email,''),(SELECT COUNT(*) FROM digital_cards d WHERE d.user_id=c.user_id) FROM contacts c LEFT JOIN users u ON u.id=c.user_id WHERE (c.name ILIKE $1 OR COALESCE(c.email,'') ILIKE $1 OR COALESCE(c.phone,'') ILIKE $1 OR COALESCE(c.source,'') ILIKE $1) AND ($2='' OR c.status=$2) ORDER BY c.created_at DESC LIMIT $3 OFFSET $4`, search, status, limit, (page-1)*limit)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load contacts"})
		return
	}
	defer rows.Close()
	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int64
		var publicID, name, source, contactStatus, notes string
		var userID *int64
		var email, phone *string
		var createdAt interface{}
		var ownerName, ownerEmail string
		var cardCount int64
		if err := rows.Scan(&id, &publicID, &userID, &name, &email, &phone, &source, &contactStatus, &notes, &createdAt, &ownerName, &ownerEmail, &cardCount); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read contacts"})
			return
		}
		items = append(items, map[string]interface{}{"id": id, "public_id": publicID, "user_id": userID, "owner_name": ownerName, "owner_email": ownerEmail, "card_count": cardCount, "name": name, "email": email, "phone": phone, "source": source, "status": contactStatus, "notes": notes, "created_at": createdAt})
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not read contacts"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": items, "pagination": map[string]int64{"page": int64(page), "limit": int64(limit), "total": total, "pages": (total + int64(limit) - 1) / int64(limit)}})
}

func (h *ResourceHandler) GetContact(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		writeJSON(w, 400, map[string]string{"error": "Invalid contact id"})
		return
	}
	var publicID, name, source, status, notes string
	var userID *int64
	var email, phone *string
	var createdAt interface{}
	var ownerName, ownerEmail string
	err = h.DB.QueryRow(r.Context(), `SELECT c.public_id::text,c.user_id,c.name,c.email,c.phone,CASE WHEN c.source='digital_card' THEN 'Digital Card' ELSE COALESCE(c.source,'Other') END,c.status,c.notes,c.created_at,COALESCE(u.name,''),COALESCE(u.email,'') FROM contacts c LEFT JOIN users u ON u.id=c.user_id WHERE c.id=$1`, id).Scan(&publicID, &userID, &name, &email, &phone, &source, &status, &notes, &createdAt, &ownerName, &ownerEmail)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "Contact not found"})
		return
	}
	cards := make([]map[string]interface{}, 0)
	if userID != nil {
		rows, e := h.DB.Query(r.Context(), `SELECT id,name,slug,status,created_at FROM digital_cards WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`, *userID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not load related cards"})
			return
		}
		defer rows.Close()
		for rows.Next() {
			var cardID int64
			var cardName, slug, status string
			var cardCreated interface{}
			if e := rows.Scan(&cardID, &cardName, &slug, &status, &cardCreated); e != nil {
				writeJSON(w, 500, map[string]string{"error": "Could not read related cards"})
				return
			}
			cards = append(cards, map[string]interface{}{"id": cardID, "name": cardName, "slug": slug, "status": status, "created_at": cardCreated})
		}
		if e := rows.Err(); e != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read related cards"})
			return
		}
	}
	writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"id": id, "public_id": publicID, "user_id": userID, "owner_name": ownerName, "owner_email": ownerEmail, "name": name, "email": email, "phone": phone, "source": source, "status": status, "notes": notes, "created_at": createdAt, "cards": cards}})
}

func (h *ResourceHandler) CreateContact(w http.ResponseWriter, r *http.Request) {
	h.saveContact(w, r, 0)
}
func (h *ResourceHandler) UpdateContact(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		writeJSON(w, 400, map[string]string{"error": "Invalid contact id"})
		return
	}
	h.saveContact(w, r, id)
}
func (h *ResourceHandler) saveContact(w http.ResponseWriter, r *http.Request, id int64) {
	creating := id == 0
	var in contactInput
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.Name) == "" {
		writeJSON(w, 400, map[string]string{"error": "Name is required"})
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.TrimSpace(in.Email)
	in.Phone = strings.TrimSpace(in.Phone)
	in.Source = strings.TrimSpace(in.Source)
	in.Status = strings.TrimSpace(in.Status)
	if in.Source == "" {
		in.Source = "Other"
	}
	if in.Status == "" {
		in.Status = "new"
	}
	if !contactSources[in.Source] {
		writeJSON(w, 400, map[string]string{"error": "Invalid source"})
		return
	}
	if !contactStatuses[in.Status] {
		writeJSON(w, 400, map[string]string{"error": "Invalid status"})
		return
	}
	if in.UserID != nil {
		var exists bool
		if err := h.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1)`, *in.UserID).Scan(&exists); err != nil || !exists {
			writeJSON(w, 400, map[string]string{"error": "Linked user not found"})
			return
		}
	}
	var err error
	if id == 0 {
		err = h.DB.QueryRow(r.Context(), `INSERT INTO contacts(user_id,name,email,phone,source,status,notes) VALUES($1,$2,NULLIF($3,''),NULLIF($4,''),$5,$6,$7) RETURNING id`, in.UserID, in.Name, in.Email, in.Phone, in.Source, in.Status, in.Notes).Scan(&id)
	} else {
		var tag interface{ RowsAffected() int64 }
		tag, err = h.DB.Exec(r.Context(), `UPDATE contacts SET user_id=$1,name=$2,email=NULLIF($3,''),phone=NULLIF($4,''),source=$5,status=$6,notes=$7 WHERE id=$8`, in.UserID, in.Name, in.Email, in.Phone, in.Source, in.Status, in.Notes, id)
		if err == nil && tag.RowsAffected() == 0 {
			writeJSON(w, 404, map[string]string{"error": "Contact not found"})
			return
		}
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not save contact"})
		return
	}
	code := 200
	if creating {
		code = 201
	}
	writeJSON(w, code, map[string]interface{}{"id": id, "name": in.Name, "email": in.Email, "phone": in.Phone, "source": in.Source, "status": in.Status, "notes": in.Notes, "user_id": in.UserID})
}
func (h *ResourceHandler) DeleteContact(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		writeJSON(w, 400, map[string]string{"error": "Invalid contact id"})
		return
	}
	tag, err := h.DB.Exec(r.Context(), `DELETE FROM contacts WHERE id=$1`, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not delete contact"})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, 404, map[string]string{"error": "Contact not found"})
		return
	}
	writeJSON(w, 200, map[string]string{"message": "Contact deleted"})
}

func (h *ResourceHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	h.createRecord(w, r, "users", false)
}
func (h *ResourceHandler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	h.createRecord(w, r, "organizations", true)
}

func (h *ResourceHandler) createRecord(w http.ResponseWriter, r *http.Request, table string, nullableEmail bool) {
	var in recordInput
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.Name) == "" {
		writeJSON(w, 400, map[string]string{"error": "Name is required"})
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.TrimSpace(in.Email)
	in.Status = normalizeStatus(in.Status)
	var id int64
	var err error
	if nullableEmail && in.Email == "" {
		err = h.DB.QueryRow(r.Context(), `INSERT INTO organizations(name,email,status) VALUES($1,NULL,$2) RETURNING id`, in.Name, in.Status).Scan(&id)
	} else {
		err = h.DB.QueryRow(r.Context(), `INSERT INTO `+table+`(name,email,status) VALUES($1,$2,$3) RETURNING id`, in.Name, in.Email, in.Status).Scan(&id)
	}
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Could not create record; check the email is unique"})
		return
	}
	writeJSON(w, 201, map[string]interface{}{"id": id, "name": in.Name, "email": in.Email, "status": in.Status})
}

func (h *ResourceHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	h.updateRecord(w, r, "users", false)
}
func (h *ResourceHandler) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
	h.updateRecord(w, r, "organizations", true)
}
func (h *ResourceHandler) updateRecord(w http.ResponseWriter, r *http.Request, table string, nullableEmail bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		writeJSON(w, 400, map[string]string{"error": "Invalid record id"})
		return
	}
	var in recordInput
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.Name) == "" {
		writeJSON(w, 400, map[string]string{"error": "Name is required"})
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.TrimSpace(in.Email)
	in.Status = normalizeStatus(in.Status)
	var tag interface{ RowsAffected() int64 }
	if nullableEmail && in.Email == "" {
		tag, err = h.DB.Exec(r.Context(), `UPDATE organizations SET name=$1,email=NULL,status=$2,updated_at=NOW() WHERE id=$3`, in.Name, in.Status, id)
	} else if table == "users" {
		tag, err = h.DB.Exec(r.Context(), `UPDATE users SET name=$1,email=$2,status=$3,updated_at=NOW() WHERE id=$4 AND deleted_at IS NULL`, in.Name, in.Email, in.Status, id)
	} else {
		tag, err = h.DB.Exec(r.Context(), `UPDATE `+table+` SET name=$1,email=$2,status=$3,updated_at=NOW() WHERE id=$4`, in.Name, in.Email, in.Status, id)
	}
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Could not update record; check the email is unique"})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, 404, map[string]string{"error": "Record not found"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"id": id, "name": in.Name, "email": in.Email, "status": in.Status})
}

func normalizeStatus(v string) string {
	if v != "inactive" && v != "suspended" {
		return "active"
	}
	return v
}
