package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type adminPermissionContextKey string

const adminPermissionsKey adminPermissionContextKey = "admin_permissions"

func AdminPermissionSet(ctx context.Context) map[string]bool {
	set, _ := ctx.Value(adminPermissionsKey).(map[string]bool)
	return set
}

// AdminAuthorization applies a fail-closed permission map to every protected admin route.
func AdminAuthorization(db *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := r.Context().Value(AdminIDKey).(int64)
			if !ok || id < 1 {
				forbidden(w, "Administrator identity is missing")
				return
			}
			var role, status, roleStatus string
			err := db.QueryRow(r.Context(), `SELECT COALESCE(r.code,a.role),a.status,COALESCE(r.status,'active') FROM admin_users a LEFT JOIN roles r ON r.id=a.role_id WHERE a.id=$1`, id).Scan(&role, &status, &roleStatus)
			if err != nil || status != "active" || (role != "super_admin" && roleStatus != "active") {
				forbidden(w, "Administrator account or assigned role is not active")
				return
			}
			required := RequiredAdminPermission(r.Method, r.URL.Path)
			if required == "" {
				forbidden(w, "No permission rule is configured for this route")
				return
			}
			if role == "super_admin" {
				next.ServeHTTP(w, r)
				return
			}
			rows, err := db.Query(r.Context(), `SELECT p.code FROM admin_users a JOIN role_permissions rp ON rp.role_id=a.role_id JOIN permissions p ON p.id=rp.permission_id WHERE a.id=$1`, id)
			if err != nil {
				forbidden(w, "Could not load administrator permissions")
				return
			}
			set := map[string]bool{}
			for rows.Next() {
				var code string
				if rows.Scan(&code) == nil {
					set[code] = true
				}
			}
			rowsErr := rows.Err()
			rows.Close()
			if rowsErr != nil {
				forbidden(w, "Could not load administrator permissions")
				return
			}
			if !set[required] {
				forbidden(w, "Permission denied")
				return
			}
			ctx := context.WithValue(r.Context(), adminPermissionsKey, set)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequiredAdminPermission(method, rawPath string) string {
	path := strings.TrimPrefix(rawPath, "/api/v1/admin")
	if path == "" {
		return ""
	}
	if path == "/dashboard" || path == "/auth/me" {
		return "dashboard.view"
	}
	if strings.HasPrefix(path, "/administration") {
		if method == http.MethodGet {
			return "administration.view"
		}
		return "administration.manage"
	}
	if strings.HasPrefix(path, "/analytics") {
		return "analytics.view"
	}
	if strings.HasPrefix(path, "/notifications") {
		if method == http.MethodGet {
			return "notifications.view"
		}
		if method == http.MethodPost {
			return "notifications.create"
		}
		return ""
	}
	if strings.HasPrefix(path, "/website/templates") {
		return resourcePermission(method, "/templates"+strings.TrimPrefix(path, "/website/templates"), "/templates", "templates")
	}
	if strings.HasPrefix(path, "/templates") {
		return resourcePermission(method, path, "/templates", "templates")
	}
	if strings.HasPrefix(path, "/payments") {
		if method == http.MethodGet {
			return "billing.view"
		}
		if method == http.MethodDelete {
			return "billing.manage"
		}
		if method == http.MethodPatch {
			return "billing.manage"
		}
		return ""
	}
	if path == "/billing/summary" || strings.HasPrefix(path, "/transactions") {
		return "billing.view"
	}
	if strings.HasPrefix(path, "/security") {
		if method == http.MethodGet {
			return "security.view"
		}
		return "security.manage"
	}
	if strings.HasPrefix(path, "/settings") {
		if method == http.MethodGet {
			return "settings.view"
		}
		return "settings.manage"
	}
	if strings.HasPrefix(path, "/plans") || strings.HasPrefix(path, "/subscriptions") {
		if method == http.MethodGet {
			return "billing.view"
		}
		return "billing.manage"
	}
	if strings.HasPrefix(path, "/website") {
		if method == http.MethodGet {
			return "website.view"
		}
		return "website.manage"
	}
	if strings.HasPrefix(path, "/qr-codes") {
		return resourcePermission(method, path, "/qr-codes", "qr")
	}
	if strings.HasPrefix(path, "/nfc") {
		return resourcePermission(method, path, "/nfc", "nfc")
	}
	if strings.HasPrefix(path, "/organizations") {
		return resourcePermission(method, path, "/organizations", "organizations")
	}
	if strings.HasPrefix(path, "/users") {
		return resourcePermission(method, path, "/users", "users")
	}
	if strings.HasPrefix(path, "/contacts") {
		return resourcePermission(method, path, "/contacts", "contacts")
	}
	if strings.HasPrefix(path, "/cards") {
		return resourcePermission(method, path, "/cards", "cards")
	}
	return ""
}

func resourcePermission(method, path, base, module string) string {
	rest := strings.Trim(strings.TrimPrefix(path, base), "/")
	if method == http.MethodGet {
		return module + ".view"
	}
	if rest == "" && method == http.MethodPost {
		return module + ".create"
	}
	if method == http.MethodDelete {
		return module + ".delete"
	}
	if method == http.MethodPut || method == http.MethodPatch {
		return module + ".update"
	}
	return ""
}

func forbidden(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
