package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	userauth "digital-card-platform/backend/internal/auth"
	"digital-card-platform/backend/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func TestUserDeleteCommittedLoginAndSessions(t *testing.T) {
	dsn := os.Getenv("WEBSITE_WRITE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set WEBSITE_WRITE_TEST_DATABASE_URL for disposable user/session checks")
	}
	t.Setenv("JWT_SECRET", "user-delete-test-only-secret")
	db, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := t.Context()
	var actor int64
	if err = db.QueryRow(ctx, `INSERT INTO admin_users(name,email,password_hash,role,status) VALUES('User delete test admin',gen_random_uuid()::text||'@example.test','test-only','super_admin','active') RETURNING id`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	defer db.Exec(context.Background(), `DELETE FROM admin_users WHERE id=$1`, actor)
	defer db.Exec(context.Background(), `DELETE FROM admin_security_activity WHERE admin_user_id=$1`, actor)
	h := &ResourceHandler{DB: db}
	w := httptest.NewRecorder()
	h.CreateUser(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"ADM USER DELETE TEST","email":"adm-user-delete-test@example.test","status":"active"}`)))
	if w.Code != 201 {
		t.Fatalf("could not create disposable user: HTTP %d", w.Code)
	}
	var created struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &created)
	if created.ID == 0 {
		t.Fatal("missing created ID")
	}
	defer db.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, created.ID)
	hash, _ := bcrypt.GenerateFromPassword([]byte("Disposable-test-password"), bcrypt.MinCost)
	if _, err = db.Exec(ctx, `UPDATE users SET password_hash=$1,email_verified_at=NOW() WHERE id=$2`, string(hash), created.ID); err != nil {
		t.Fatal(err)
	}
	var other int64
	if err = db.QueryRow(ctx, `INSERT INTO users(name,email,status) VALUES('Other session fixture',gen_random_uuid()::text||'@example.test','active') RETURNING id`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	defer db.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, other)
	login := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		(&userauth.UserAuthHandler{DB: db}).Login(w, httptest.NewRequest("POST", "/auth/login", strings.NewReader(`{"email":"adm-user-delete-test@example.test","password":"Disposable-test-password"}`)))
		return w
	}
	w = login()
	if w.Code != 200 {
		t.Fatalf("fixture login: HTTP %d", w.Code)
	}
	var session struct {
		Token string `json:"token"`
	}
	json.Unmarshal(w.Body.Bytes(), &session)
	if session.Token == "" {
		t.Fatal("fixture login did not create session")
	}
	otherToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, middleware.UserClaims{UserID: other, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}).SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		t.Fatal("fixture token failed")
	}
	adminToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, middleware.AdminClaims{AdminID: actor, Role: "super_admin", RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}).SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		t.Fatal("fixture admin token failed")
	}
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	check := func(handler http.Handler, token string, want int) {
		t.Helper()
		r := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("session: got %d, want %d", w.Code, want)
		}
	}
	customerHandler := middleware.UserAuth(middleware.ActiveCustomerAccount(db)(ok))
	adminHandler := middleware.AdminAuth(middleware.AdminAuthorization(db)(ok))
	check(customerHandler, session.Token, 204)
	check(customerHandler, otherToken, 204)
	check(adminHandler, adminToken, 204)
	w = userDeleteRequest(h, fmt.Sprint(created.ID), actor)
	if w.Code != 200 {
		t.Fatalf("delete: HTTP %d", w.Code)
	}
	if w = login(); w.Code != 401 {
		t.Fatalf("deleted login: HTTP %d", w.Code)
	}
	check(customerHandler, session.Token, 401)
	check(customerHandler, otherToken, 204)
	check(adminHandler, adminToken, 204)
	fresh, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	var deleted bool
	if err = fresh.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM users WHERE id=$1`, created.ID).Scan(&deleted); err != nil || !deleted {
		t.Fatal("deletion not persisted")
	}
	for range 2 {
		w = httptest.NewRecorder()
		(&ResourceHandler{DB: fresh}).ListUsers(w, httptest.NewRequest("GET", "/?search=adm-user-delete-test%40example.test", nil))
		var list struct {
			Data []any `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &list)
		if w.Code != 200 || len(list.Data) != 0 {
			t.Fatal("deleted user returned after refresh")
		}
	}
}

func userDeleteRequest(h *ResourceHandler, id string, adminID int64) *httptest.ResponseRecorder {
	r := httptest.NewRequest("DELETE", "/api/v1/admin/users/"+id, nil)
	rc := chi.NewRouteContext()
	rc.URLParams.Add("id", id)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rc)
	ctx = context.WithValue(ctx, middleware.AdminIDKey, adminID)
	w := httptest.NewRecorder()
	h.DeleteUser(w, r.WithContext(ctx))
	return w
}

func TestUserDeleteValidationAndSecurity(t *testing.T) {
	for _, id := range []string{"bad", "0", "-1"} {
		if w := userDeleteRequest(&ResourceHandler{}, id, 1); w.Code != 400 {
			t.Fatal(w.Code)
		}
	}
	if w := userDeleteRequest(&ResourceHandler{}, "1", 0); w.Code != 401 {
		t.Fatal(w.Code)
	}
	db, err := pgxpool.New(t.Context(), "postgres://test:test@127.0.0.1:1/test?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	w := userDeleteRequest(&ResourceHandler{DB: db}, "1", 1)
	if w.Code != 500 || !strings.Contains(w.Body.String(), "Could not delete user.") || strings.Contains(w.Body.String(), "pool") {
		t.Fatalf("unsafe failure: %d %s", w.Code, w.Body.String())
	}
	never := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("unauthorized handler") })
	path := "/api/v1/admin/users/1"
	w = httptest.NewRecorder()
	middleware.AdminAuth(never).ServeHTTP(w, httptest.NewRequest("DELETE", path, nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	r := httptest.NewRequest("DELETE", path, nil)
	r = r.WithContext(context.WithValue(r.Context(), middleware.AdminIDKey, int64(1)))
	w = httptest.NewRecorder()
	middleware.AdminAuthorization(db)(never).ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	if middleware.RequiredAdminPermission("DELETE", path) != "users.delete" {
		t.Fatal("wrong permission")
	}
}

func TestUserDeleteDatabaseLifecycle(t *testing.T) {
	dsn := os.Getenv("WEBSITE_WRITE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set WEBSITE_WRITE_TEST_DATABASE_URL for rollback-only user checks")
	}
	db, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(t.Context(), sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	id := func(sql string, args ...any) int64 {
		t.Helper()
		var n int64
		if err := tx.QueryRow(t.Context(), sql, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	admin := id(`INSERT INTO admin_users(name,email,password_hash,role,status) VALUES('User delete test',gen_random_uuid()::text||'@example.test','test-only','super_admin','active') RETURNING id`)
	h := &ResourceHandler{DB: tx}
	if w := userDeleteRequest(h, "9223372036854775807", admin); w.Code != 404 {
		t.Fatalf("missing: %d", w.Code)
	}
	self := id(`INSERT INTO users(name,email) SELECT 'Self test',email FROM admin_users WHERE id=$1 RETURNING id`, admin)
	if w := userDeleteRequest(h, fmt.Sprint(self), admin); w.Code != 409 || !strings.Contains(w.Body.String(), "currently authenticated") {
		t.Fatalf("self: %d %s", w.Code, w.Body.String())
	}
	user := id(`INSERT INTO users(name,email,status,password_hash) VALUES('ADM USER DELETE TEST',gen_random_uuid()::text||'@example.test','active','test-only') RETURNING id`)
	other := id(`INSERT INTO users(name,email) VALUES('Other user',gen_random_uuid()::text||'@example.test') RETURNING id`)
	org := id(`INSERT INTO organizations(name) VALUES('User delete fixture') RETURNING id`)
	card := id(`INSERT INTO digital_cards(user_id,organization_id,name,slug,status) VALUES($1,$2,'Preserved card',gen_random_uuid()::text,'published') RETURNING id`, user, org)
	plan := id(`INSERT INTO website_pricing_plans(name,code) VALUES('User delete fixture',gen_random_uuid()::text) RETURNING id`)
	sub := id(`INSERT INTO subscriptions(user_id,organization_id,plan_id,status) VALUES($1,$2,$3,'active') RETURNING id`, user, org, plan)
	payment := id(`INSERT INTO payments(user_id,organization_id,subscription_id,plan_id,provider_order_id,amount,currency) VALUES($1,$2,$3,$4,gen_random_uuid()::text,10,'INR') RETURNING id`, user, org, sub, plan)
	exec(`INSERT INTO payment_transactions(payment_id,event_type,amount,currency) VALUES($1,'fixture',10,'INR')`, payment)
	exec(`INSERT INTO contacts(user_id,name) VALUES($1,'Preserved contact')`, user)
	exec(`INSERT INTO qr_codes(user_id,card_id,organization_id,code_value) VALUES($1,$2,$3,gen_random_uuid()::text)`, user, card, org)
	exec(`INSERT INTO nfc_tags(user_id,card_id,organization_id,uid) VALUES($1,$2,$3,gen_random_uuid()::text)`, user, card, org)
	exec(`INSERT INTO analytics_events(user_id,card_id,event_type) VALUES($1,$2,'fixture')`, user, card)
	exec(`INSERT INTO auth_otp_codes(subject_type,subject_id,destination,purpose,code_hash,verification_token_hash,expires_at) SELECT 'customer',id,email,'customer_password_reset','test-only','test-only',NOW()+INTERVAL '10 minutes' FROM users WHERE id IN($1,$2)`, user, other)
	exec(`INSERT INTO auth_otp_codes(subject_type,subject_id,destination,purpose,code_hash,expires_at) SELECT 'admin',id,email,'admin_password_reset','test-only',NOW()+INTERVAL '10 minutes' FROM admin_users WHERE id=$1`, admin)
	snapshot := func() string {
		t.Helper()
		var s string
		err := tx.QueryRow(t.Context(), `SELECT jsonb_build_object('cards',(SELECT jsonb_agg(to_jsonb(c)-'status'-'updated_at') FROM digital_cards c WHERE user_id=$1),'subscriptions',(SELECT jsonb_agg(to_jsonb(s)-'status'-'updated_at') FROM subscriptions s WHERE user_id=$1),'payments',(SELECT jsonb_agg(to_jsonb(p)) FROM payments p WHERE user_id=$1),'history',(SELECT jsonb_agg(to_jsonb(p)) FROM payment_transactions p WHERE payment_id=$2),'contacts',(SELECT jsonb_agg(to_jsonb(c)) FROM contacts c WHERE user_id=$1),'qr',(SELECT jsonb_agg(to_jsonb(q)) FROM qr_codes q WHERE user_id=$1),'nfc',(SELECT jsonb_agg(to_jsonb(n)) FROM nfc_tags n WHERE user_id=$1),'analytics',(SELECT jsonb_agg(to_jsonb(a)) FROM analytics_events a WHERE user_id=$1),'admin',(SELECT to_jsonb(a) FROM admin_users a WHERE id=$3),'other',(SELECT to_jsonb(u) FROM users u WHERE id=$4))::text`, user, payment, admin, other).Scan(&s)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	before := snapshot()
	w := userDeleteRequest(h, fmt.Sprint(user), admin)
	if w.Code != 200 {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	if before != snapshot() {
		t.Fatal("unrelated account or historical data changed")
	}
	var safe bool
	if err = tx.QueryRow(t.Context(), `SELECT deleted_at IS NOT NULL AND status='inactive' AND password_hash IS NULL FROM users WHERE id=$1`, user).Scan(&safe); err != nil || !safe {
		t.Fatalf("account not disabled: %v", err)
	}
	if id(`SELECT COUNT(*) FROM digital_cards WHERE id=$1 AND status='archived'`, card) != 1 {
		t.Fatal("card not retained and archived")
	}
	if id(`SELECT COUNT(*) FROM subscriptions WHERE id=$1 AND status='cancelled'`, sub) != 1 {
		t.Fatal("subscription not retained")
	}
	if id(`SELECT COUNT(*) FROM auth_otp_codes WHERE subject_type='customer' AND subject_id=$1 AND used_at IS NOT NULL AND verification_token_used_at IS NOT NULL`, user) != 1 {
		t.Fatal("target OTP not invalidated")
	}
	if id(`SELECT COUNT(*) FROM auth_otp_codes WHERE ((subject_type='customer' AND subject_id=$1) OR (subject_type='admin' AND subject_id=$2)) AND used_at IS NULL`, other, admin) != 2 {
		t.Fatal("other credentials invalidated")
	}
	for _, status := range []string{"", "inactive", "deleted"} {
		for range 2 {
			w := httptest.NewRecorder()
			h.ListUsers(w, httptest.NewRequest("GET", "/?status="+status, nil))
			if w.Code != 200 {
				t.Fatal(w.Body.String())
			}
			var out struct {
				Data []struct {
					ID int64 `json:"id"`
				} `json:"data"`
			}
			json.Unmarshal(w.Body.Bytes(), &out)
			for _, row := range out.Data {
				if row.ID == user {
					t.Fatal("deleted user in normal list")
				}
			}
		}
	}
	rc := chi.NewRouteContext()
	rc.URLParams.Add("id", fmt.Sprint(user))
	r := httptest.NewRequest("PUT", "/", strings.NewReader(`{"name":"Reactivated","email":"reactivated@example.test","status":"active"}`))
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))
	w = httptest.NewRecorder()
	h.UpdateUser(w, r)
	if w.Code != 404 && w.Code != 409 {
		t.Fatalf("deleted user reactivated: %d", w.Code)
	}
}
