package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	appmiddleware "digital-card-platform/backend/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdministrationHandler struct{ DB *pgxpool.Pool }

func (h *AdministrationHandler) Summary(w http.ResponseWriter, r *http.Request) {
	var x struct{ TotalUsers, AdminUsers, ActiveUsers, UserTypes, Roles, Permissions int64 }
	err := h.DB.QueryRow(r.Context(), `SELECT (SELECT COUNT(*) FROM users),(SELECT COUNT(*) FROM admin_users),(SELECT COUNT(*) FROM users WHERE status='active'),(SELECT COUNT(*) FROM user_types),(SELECT COUNT(*) FROM roles),(SELECT COUNT(*) FROM permissions)`).Scan(&x.TotalUsers, &x.AdminUsers, &x.ActiveUsers, &x.UserTypes, &x.Roles, &x.Permissions)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load Administration summary"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": map[string]int64{"total_users": x.TotalUsers, "admin_users": x.AdminUsers, "active_users": x.ActiveUsers, "user_types": x.UserTypes, "roles": x.Roles, "permissions": x.Permissions}})
}

func (h *AdministrationHandler) Users(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.createUser(w, r)
		return
	}
	page, limit := pagination(r)
	search := "%" + strings.TrimSpace(r.URL.Query().Get("search")) + "%"
	typ := r.URL.Query().Get("user_type_id")
	role := r.URL.Query().Get("role_id")
	status := r.URL.Query().Get("status")
	var total int64
	err := h.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM users u WHERE ($1='' OR u.name ILIKE $2 OR u.email ILIKE $2) AND ($3='' OR u.user_type_id::text=$3) AND ($4='' OR u.role_id::text=$4) AND ($5='' OR ($5='deleted' AND u.deleted_at IS NOT NULL) OR ($5<>'deleted' AND u.deleted_at IS NULL AND u.status=$5))`, r.URL.Query().Get("search"), search, typ, role, status).Scan(&total)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not count users"})
		return
	}
	rows, err := h.DB.Query(r.Context(), `SELECT u.id,u.public_id::text,u.name,u.email,COALESCE(ut.name,''),COALESCE(ut.id,0),COALESCE(ro.name,''),COALESCE(ro.id,0),u.status,u.created_at,u.deleted_at FROM users u LEFT JOIN user_types ut ON ut.id=u.user_type_id LEFT JOIN roles ro ON ro.id=u.role_id WHERE ($1='' OR u.name ILIKE $2 OR u.email ILIKE $2) AND ($3='' OR u.user_type_id::text=$3) AND ($4='' OR u.role_id::text=$4) AND ($5='' OR ($5='deleted' AND u.deleted_at IS NOT NULL) OR ($5<>'deleted' AND u.deleted_at IS NULL AND u.status=$5)) ORDER BY u.created_at DESC LIMIT $6 OFFSET $7`, r.URL.Query().Get("search"), search, typ, role, status, limit, (page-1)*limit)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load users"})
		return
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var id, utID, roleID int64
		var publicID, name, email, utName, roleName, status string
		var created interface{}
		var deletedAt *time.Time
		if rows.Scan(&id, &publicID, &name, &email, &utName, &utID, &roleName, &roleID, &status, &created, &deletedAt) != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read users"})
			return
		}
		var typeVal, roleVal interface{}
		if utID > 0 {
			typeVal = utID
		}
		if roleID > 0 {
			roleVal = roleID
		}
		if deletedAt != nil {
			status = "deleted"
		}
		out = append(out, map[string]interface{}{"id": id, "public_id": publicID, "name": name, "email": email, "user_type": utName, "user_type_id": typeVal, "role": roleName, "role_id": roleVal, "status": status, "created_at": created, "deleted_at": deletedAt})
	}
	if rows.Err() != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not read users"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": out, "pagination": map[string]int64{"page": int64(page), "limit": int64(limit), "total": total, "pages": (total + int64(limit) - 1) / int64(limit)}})
}

func (h *AdministrationHandler) User(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		h.deleteUser(w, r)
		return
	}
	if r.Method == http.MethodPut || r.Method == http.MethodPatch {
		h.updateUser(w, r)
		return
	}
	id, e := adminRouteID(r)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid user id"})
		return
	}
	var publicID, name, email, typeName, roleName, status string
	var typeID, roleID *int64
	var created interface{}
	var deletedAt *time.Time
	e = h.DB.QueryRow(r.Context(), `SELECT u.public_id::text,u.name,u.email,u.user_type_id,u.role_id,COALESCE(t.name,''),COALESCE(ro.name,''),u.status,u.created_at,u.deleted_at FROM users u LEFT JOIN user_types t ON t.id=u.user_type_id LEFT JOIN roles ro ON ro.id=u.role_id WHERE u.id=$1`, id).Scan(&publicID, &name, &email, &typeID, &roleID, &typeName, &roleName, &status, &created, &deletedAt)
	if e != nil {
		writeJSON(w, 404, map[string]string{"error": "User not found"})
		return
	}
	if deletedAt != nil {
		status = "deleted"
	}
	writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"id": id, "public_id": publicID, "name": name, "email": email, "user_type_id": typeID, "user_type": typeName, "role_id": roleID, "role": roleName, "status": status, "created_at": created, "deleted_at": deletedAt}})
}

type adminUserDraft struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	Status     string `json:"status"`
	UserTypeID *int64 `json:"user_type_id"`
	RoleID     *int64 `json:"role_id"`
}

func validateUserDraft(d *adminUserDraft) bool {
	d.Name = strings.TrimSpace(d.Name)
	d.Email = strings.ToLower(strings.TrimSpace(d.Email))
	if d.Name == "" || len(d.Name) > 150 || d.Email == "" || len(d.Email) > 255 {
		return false
	}
	if d.Status != "active" && d.Status != "inactive" && d.Status != "suspended" {
		return false
	}
	return true
}
func (h *AdministrationHandler) createUser(w http.ResponseWriter, r *http.Request) {
	var d adminUserDraft
	if json.NewDecoder(r.Body).Decode(&d) != nil || !validateUserDraft(&d) {
		writeJSON(w, 400, map[string]string{"error": "Valid name, email and status are required"})
		return
	}
	if !h.validAssignments(r, &d) {
		writeJSON(w, 400, map[string]string{"error": "Selected user type or role was not found or enabled"})
		return
	}
	var id int64
	if e := h.DB.QueryRow(r.Context(), `INSERT INTO users(name,email,status,user_type_id,role_id) VALUES($1,$2,$3,$4,$5) RETURNING id`, d.Name, d.Email, d.Status, d.UserTypeID, d.RoleID).Scan(&id); e != nil {
		writeJSON(w, 409, map[string]string{"error": "Could not create user; check that the email is unique"})
		return
	}
	auditAdmin(r, h.DB, "administration_user_created", "user", id, "Created platform user")
	writeJSON(w, 201, map[string]interface{}{"data": map[string]interface{}{"id": id, "name": d.Name, "email": d.Email, "status": d.Status, "user_type_id": d.UserTypeID, "role_id": d.RoleID}})
}
func (h *AdministrationHandler) updateUser(w http.ResponseWriter, r *http.Request) {
	id, e := adminRouteID(r)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid user id"})
		return
	}
	var deleted bool
	if e = h.DB.QueryRow(r.Context(), `SELECT deleted_at IS NOT NULL FROM users WHERE id=$1`, id).Scan(&deleted); e != nil {
		writeJSON(w, 404, map[string]string{"error": "User not found"})
		return
	}
	if deleted {
		writeJSON(w, 409, map[string]string{"error": "Deleted users cannot be edited or reactivated"})
		return
	}
	var d adminUserDraft
	if json.NewDecoder(r.Body).Decode(&d) != nil || !validateUserDraft(&d) {
		writeJSON(w, 400, map[string]string{"error": "Valid name, email and status are required"})
		return
	}
	if !h.validAssignments(r, &d) {
		writeJSON(w, 400, map[string]string{"error": "Selected user type or role was not found or enabled"})
		return
	}
	tag, e := h.DB.Exec(r.Context(), `UPDATE users SET name=$1,email=$2,status=$3,user_type_id=$4,role_id=$5,updated_at=NOW() WHERE id=$6 AND deleted_at IS NULL`, d.Name, d.Email, d.Status, d.UserTypeID, d.RoleID, id)
	if e != nil {
		writeJSON(w, 409, map[string]string{"error": "Could not update user; check that the email is unique"})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, 404, map[string]string{"error": "User not found"})
		return
	}
	auditAdmin(r, h.DB, "administration_user_updated", "user", id, "Updated platform user")
	writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"id": id, "name": d.Name, "email": d.Email, "status": d.Status, "user_type_id": d.UserTypeID, "role_id": d.RoleID}})
}

// deleteUser removes customer login and public exposure while retaining the
// card, analytics, subscription, and payment records for audit/history.
func (h *AdministrationHandler) deleteUser(w http.ResponseWriter, r *http.Request) {
	deletePlatformUser(w, r, h.DB)
}
func (h *AdministrationHandler) validAssignments(r *http.Request, d *adminUserDraft) bool {
	var ok bool
	if d.UserTypeID != nil {
		if h.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM user_types WHERE id=$1 AND status='active')`, *d.UserTypeID).Scan(&ok) != nil || !ok {
			return false
		}
	}
	if d.RoleID != nil {
		if h.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM roles WHERE id=$1 AND status='active')`, *d.RoleID).Scan(&ok) != nil || !ok {
			return false
		}
	}
	return true
}

func (h *AdministrationHandler) UserTypes(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.saveUserType(w, r, 0)
		return
	}
	rows, e := h.DB.Query(r.Context(), `SELECT id,name,code,description,status,created_at,updated_at FROM user_types ORDER BY name`)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load user types"})
		return
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var id int64
		var name, code, desc, status string
		var created, updated interface{}
		if rows.Scan(&id, &name, &code, &desc, &status, &created, &updated) != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read user types"})
			return
		}
		out = append(out, map[string]interface{}{"id": id, "name": name, "code": code, "description": desc, "status": status, "created_at": created, "updated_at": updated})
	}
	writeJSON(w, 200, map[string]interface{}{"data": out})
}
func (h *AdministrationHandler) UserType(w http.ResponseWriter, r *http.Request) {
	id, e := adminRouteID(r)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid user type id"})
		return
	}
	if r.Method == http.MethodPut || r.Method == http.MethodPatch {
		h.saveUserType(w, r, id)
		return
	}
	var name, code, desc, status string
	var created, updated interface{}
	e = h.DB.QueryRow(r.Context(), `SELECT name,code,description,status,created_at,updated_at FROM user_types WHERE id=$1`, id).Scan(&name, &code, &desc, &status, &created, &updated)
	if e != nil {
		writeJSON(w, 404, map[string]string{"error": "User type not found"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"id": id, "name": name, "code": code, "description": desc, "status": status, "created_at": created, "updated_at": updated}})
}
func (h *AdministrationHandler) saveUserType(w http.ResponseWriter, r *http.Request, id int64) {
	var d struct {
		Name        string `json:"name"`
		Code        string `json:"code"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}
	if json.NewDecoder(r.Body).Decode(&d) != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid user type"})
		return
	}
	d.Name = strings.TrimSpace(d.Name)
	d.Code = normalizeCode(d.Code)
	d.Description = strings.TrimSpace(d.Description)
	if d.Name == "" || d.Code == "" {
		writeJSON(w, 400, map[string]string{"error": "Name and code are required"})
		return
	}
	if d.Status == "" {
		d.Status = "active"
	}
	if d.Status != "active" && d.Status != "disabled" {
		writeJSON(w, 400, map[string]string{"error": "Status must be active or disabled"})
		return
	}
	var e error
	if id == 0 {
		e = h.DB.QueryRow(r.Context(), `INSERT INTO user_types(name,code,description,status) VALUES($1,$2,$3,$4) RETURNING id`, d.Name, d.Code, d.Description, d.Status).Scan(&id)
	} else {
		e0, execErr := h.DB.Exec(r.Context(), `UPDATE user_types SET name=$1,code=$2,description=$3,status=$4,updated_at=NOW() WHERE id=$5`, d.Name, d.Code, d.Description, d.Status, id)
		e = execErr
		if e == nil && e0.RowsAffected() == 0 {
			writeJSON(w, 404, map[string]string{"error": "User type not found"})
			return
		}
	}
	if e != nil {
		writeJSON(w, 409, map[string]string{"error": "Could not save user type; name/code may already exist"})
		return
	}
	auditAdmin(r, h.DB, "administration_user_type_saved", "user_type", id, "Saved user type")
	writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"id": id, "name": d.Name, "code": d.Code, "description": d.Description, "status": d.Status}})
}

func (h *AdministrationHandler) Roles(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.saveRole(w, r, 0)
		return
	}
	rows, e := h.DB.Query(r.Context(), `SELECT r.id,r.name,r.code,r.description,r.status,r.is_system,r.created_at,(SELECT COUNT(*) FROM users u WHERE u.role_id=r.id)+(SELECT COUNT(*) FROM admin_users a WHERE a.role_id=r.id),(SELECT COUNT(*) FROM role_permissions rp WHERE rp.role_id=r.id) FROM roles r ORDER BY r.name`)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load roles"})
		return
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var id, users, perms int64
		var name, code, desc, status string
		var system bool
		var created interface{}
		if rows.Scan(&id, &name, &code, &desc, &status, &system, &created, &users, &perms) != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read roles"})
			return
		}
		out = append(out, map[string]interface{}{"id": id, "name": name, "code": code, "description": desc, "status": status, "is_system": system, "created_at": created, "users_count": users, "permissions_count": perms})
	}
	writeJSON(w, 200, map[string]interface{}{"data": out})
}
func (h *AdministrationHandler) Role(w http.ResponseWriter, r *http.Request) {
	id, e := adminRouteID(r)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid role id"})
		return
	}
	if r.Method == http.MethodPut || r.Method == http.MethodPatch {
		h.saveRole(w, r, id)
		return
	}
	var name, code, desc, status string
	var system bool
	e = h.DB.QueryRow(r.Context(), `SELECT name,code,description,status,is_system FROM roles WHERE id=$1`, id).Scan(&name, &code, &desc, &status, &system)
	if e != nil {
		writeJSON(w, 404, map[string]string{"error": "Role not found"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"id": id, "name": name, "code": code, "description": desc, "status": status, "is_system": system}})
}
func (h *AdministrationHandler) saveRole(w http.ResponseWriter, r *http.Request, id int64) {
	var d struct {
		Name        string `json:"name"`
		Code        string `json:"code"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}
	if json.NewDecoder(r.Body).Decode(&d) != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid role"})
		return
	}
	d.Name = strings.TrimSpace(d.Name)
	d.Code = normalizeCode(d.Code)
	d.Description = strings.TrimSpace(d.Description)
	if d.Name == "" {
		writeJSON(w, 400, map[string]string{"error": "Role name is required"})
		return
	}
	if d.Status == "" {
		d.Status = "active"
	}
	if d.Status != "active" && d.Status != "disabled" {
		writeJSON(w, 400, map[string]string{"error": "Status must be active or disabled"})
		return
	}
	if id == 0 && d.Code == "" {
		writeJSON(w, 400, map[string]string{"error": "Role code is required"})
		return
	}
	var e error
	if id == 0 {
		e = h.DB.QueryRow(r.Context(), `INSERT INTO roles(name,code,description,status) VALUES($1,$2,$3,$4) RETURNING id`, d.Name, d.Code, d.Description, d.Status).Scan(&id)
	} else {
		var current string
		if e = h.DB.QueryRow(r.Context(), `SELECT code FROM roles WHERE id=$1`, id).Scan(&current); e != nil {
			writeJSON(w, 404, map[string]string{"error": "Role not found"})
			return
		}
		if current == "super_admin" && (!isSuperAdmin(r, h.DB) || d.Status != "active") {
			writeJSON(w, 403, map[string]string{"error": "The super_admin role must remain active and is managed only by a super administrator"})
			return
		}
		tag, ex := h.DB.Exec(r.Context(), `UPDATE roles SET name=$1,description=$2,status=$3,updated_at=NOW() WHERE id=$4`, d.Name, d.Description, d.Status, id)
		e = ex
		if e == nil && tag.RowsAffected() == 0 {
			writeJSON(w, 404, map[string]string{"error": "Role not found"})
			return
		}
	}
	if e != nil {
		writeJSON(w, 409, map[string]string{"error": "Could not save role; code may already exist"})
		return
	}
	auditAdmin(r, h.DB, "administration_role_saved", "role", id, "Saved role")
	writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"id": id, "name": d.Name, "code": d.Code, "description": d.Description, "status": d.Status}})
}
func (h *AdministrationHandler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	id, e := adminRouteID(r)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid role id"})
		return
	}
	var code string
	if e = h.DB.QueryRow(r.Context(), `SELECT code FROM roles WHERE id=$1`, id).Scan(&code); e != nil {
		writeJSON(w, 404, map[string]string{"error": "Role not found"})
		return
	}
	if code == "super_admin" {
		writeJSON(w, 400, map[string]string{"error": "The super_admin role cannot be deleted"})
		return
	}
	var count int64
	if e = h.DB.QueryRow(r.Context(), `SELECT (SELECT COUNT(*) FROM users WHERE role_id=$1)+(SELECT COUNT(*) FROM admin_users WHERE role_id=$1)`, id).Scan(&count); e != nil || count > 0 {
		writeJSON(w, 409, map[string]string{"error": "Role is assigned to users and cannot be deleted"})
		return
	}
	if _, e = h.DB.Exec(r.Context(), `DELETE FROM roles WHERE id=$1`, id); e != nil {
		writeJSON(w, 409, map[string]string{"error": "Could not delete role"})
		return
	}
	auditAdmin(r, h.DB, "administration_role_deleted", "role", id, "Deleted role")
	writeJSON(w, 200, map[string]string{"message": "Role deleted"})
}

func (h *AdministrationHandler) Permissions(w http.ResponseWriter, r *http.Request) {
	rows, e := h.DB.Query(r.Context(), `SELECT id,module,name,code,description FROM permissions ORDER BY module,name`)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load permissions"})
		return
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var id int64
		var module, name, code, desc string
		if rows.Scan(&id, &module, &name, &code, &desc) != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read permissions"})
			return
		}
		out = append(out, map[string]interface{}{"id": id, "module": module, "name": name, "code": code, "description": desc})
	}
	writeJSON(w, 200, map[string]interface{}{"data": out})
}
func (h *AdministrationHandler) RolePermissions(w http.ResponseWriter, r *http.Request) {
	id, e := adminRouteID(r)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid role id"})
		return
	}
	var code string
	if e = h.DB.QueryRow(r.Context(), `SELECT code FROM roles WHERE id=$1`, id).Scan(&code); e != nil {
		writeJSON(w, 404, map[string]string{"error": "Role not found"})
		return
	}
	if r.Method == http.MethodGet {
		rows, e := h.DB.Query(r.Context(), `SELECT p.code FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE rp.role_id=$1 ORDER BY p.module,p.name`, id)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not load role permissions"})
			return
		}
		defer rows.Close()
		codes := []string{}
		for rows.Next() {
			var c string
			if rows.Scan(&c) == nil {
				codes = append(codes, c)
			}
		}
		writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"role_id": id, "role_code": code, "permissions": codes}})
		return
	}
	if code == "super_admin" && !isSuperAdmin(r, h.DB) {
		writeJSON(w, 403, map[string]string{"error": "Only a super administrator can change super_admin permissions"})
		return
	}
	var in struct {
		Permissions []string `json:"permissions"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid permission list"})
		return
	}
	allowed := appmiddleware.AdminPermissionSet(r.Context())
	if !isSuperAdmin(r, h.DB) {
		for _, p := range in.Permissions {
			if !allowed[p] {
				writeJSON(w, 403, map[string]string{"error": "You cannot grant a permission you do not hold"})
				return
			}
		}
	}
	tx, e := h.DB.Begin(r.Context())
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not save role permissions"})
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	if _, e = tx.Exec(r.Context(), `DELETE FROM role_permissions WHERE role_id=$1`, id); e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not save role permissions"})
		return
	}
	for _, p := range uniqueStrings(in.Permissions) {
		tag, ex := tx.Exec(r.Context(), `INSERT INTO role_permissions(role_id,permission_id) SELECT $1,id FROM permissions WHERE code=$2 ON CONFLICT DO NOTHING`, id, p)
		if ex != nil || tag.RowsAffected() == 0 {
			writeJSON(w, 400, map[string]string{"error": "Unknown permission code: " + p})
			return
		}
	}
	if e = tx.Commit(r.Context()); e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not save role permissions"})
		return
	}
	auditAdmin(r, h.DB, "administration_role_permissions_updated", "role", id, "Updated role permissions")
	writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"role_id": id, "permissions": uniqueStrings(in.Permissions)}})
}

func adminRouteID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		return 0, strconv.ErrSyntax
	}
	return id, nil
}
func normalizeCode(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "_")
	return s
}
func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
func isSuperAdmin(r *http.Request, db *pgxpool.Pool) bool {
	id, ok := r.Context().Value(appmiddleware.AdminIDKey).(int64)
	if !ok {
		return false
	}
	var yes bool
	return db.QueryRow(r.Context(), `SELECT COALESCE(r.code,a.role)='super_admin' FROM admin_users a LEFT JOIN roles r ON r.id=a.role_id WHERE a.id=$1`, id).Scan(&yes) == nil && yes
}
