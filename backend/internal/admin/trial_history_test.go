package admin

import (
	"context"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type trialQueryTracer struct{ t *testing.T }

func (v trialQueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	return ctx
}
func (v trialQueryTracer) TraceQueryEnd(_ context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if data.Err != nil {
		v.t.Logf("isolated query error: %v", data.Err)
	}
}

func TestAdminTrialMarkerSurvivesCancellationAndEdits(t *testing.T) {
	dsn := os.Getenv("PAYMENT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL test connection")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.Tracer = trialQueryTracer{t}
	db, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	for _, table := range []string{"users", "website_pricing_plans", "subscriptions"} {
		if _, err = db.Exec(t.Context(), fmt.Sprintf("CREATE TEMP TABLE %s (LIKE public.%s INCLUDING DEFAULTS INCLUDING CONSTRAINTS)", table, table)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(t.Context(), `CREATE TEMP SEQUENCE admin_trial_seq; ALTER TABLE subscriptions ALTER COLUMN id SET DEFAULT nextval('pg_temp.admin_trial_seq'); INSERT INTO users(id,name,email,status) VALUES(1,'Trial QA','trial@example.invalid','active'); INSERT INTO website_pricing_plans(id,name,code,price,currency,billing_period,trial_mode) VALUES(1,'Trial QA','trialqa',25,'INR','monthly','unlimited')`); err != nil {
		t.Fatal(err)
	}
	h := &PlansBillingHandler{DB: db}
	request := func(status string, id int64, changeStatus bool) {
		body := fmt.Sprintf(`{"plan_id":1,"user_id":1,"status":"%s"}`, status)
		if changeStatus {
			body = fmt.Sprintf(`{"status":"%s"}`, status)
		}
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		if id > 0 {
			route := chi.NewRouteContext()
			route.URLParams.Add("id", fmt.Sprint(id))
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
		}
		w := httptest.NewRecorder()
		if changeStatus {
			h.SubscriptionStatus(w, r)
		} else {
			h.saveSubscription(w, r, id)
		}
		if w.Code >= 300 {
			t.Fatalf("admin save: %d %s", w.Code, w.Body)
		}
	}
	request("trial", 0, false)
	request("cancelled", 1, true)
	request("cancelled", 1, false)
	var used bool
	if err = db.QueryRow(t.Context(), `SELECT is_trial FROM subscriptions WHERE id=1`).Scan(&used); err != nil {
		t.Fatal(err)
	}
	if !used {
		t.Fatal("admin unlimited trial history lost after cancellation/edit")
	}
}
