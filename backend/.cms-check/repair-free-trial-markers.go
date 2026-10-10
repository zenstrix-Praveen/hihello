// Correct only three identified local rows marked by the initial overbroad QA
// backfill. The deployed migration excludes free plans and never clears history.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"os"
	"time"
)

func main() {
	_ = godotenv.Load()
	if os.Getenv("APP_ENV") != "local" {
		panic("Local only")
	}
	cfg, e := pgx.ParseConfig(os.Getenv("DATABASE_URL"))
	if e != nil || (cfg.Host != "localhost" && cfg.Host != "127.0.0.1") {
		panic("Local DB only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, e := pgx.ConnectConfig(ctx, cfg)
	if e != nil {
		panic("Local DB unavailable")
	}
	defer db.Close(ctx)
	tx, e := db.Begin(ctx)
	if e != nil {
		panic("Transaction unavailable")
	}
	defer tx.Rollback(ctx)
	var before, after int
	const paid = `SELECT COUNT(*) FROM subscriptions s JOIN website_pricing_plans p ON p.id=s.plan_id WHERE s.user_id=6 AND s.is_trial AND p.price>0 AND p.pricing_type<>'free'`
	if tx.QueryRow(ctx, paid).Scan(&before) != nil {
		panic("Audit failed")
	}
	tag, e := tx.Exec(ctx, `UPDATE subscriptions s SET is_trial=FALSE FROM website_pricing_plans p WHERE s.id IN (43,44,45) AND s.user_id=6 AND s.plan_id=31 AND s.amount=0 AND s.status='cancelled' AND s.updated_at<'2026-10-09T14:10:00+05:30' AND s.is_trial AND p.id=s.plan_id AND p.price=0 AND p.pricing_type='free'`)
	if e != nil || tag.RowsAffected() != 3 {
		panic("Identified legacy-row guard mismatch")
	}
	if tx.QueryRow(ctx, paid).Scan(&after) != nil || before != after {
		panic("Paid trial markers changed")
	}
	if tx.Commit(ctx) != nil {
		panic("Commit failed")
	}
	result := map[string]any{"corrected_local_free_subscription_ids": []int{43, 44, 45}, "paid_trial_markers_before": before, "paid_trial_markers_after": after, "note": "Only initial QA backfill correction; subscription/payment history preserved"}
	b, _ := json.MarshalIndent(result, "", "  ")
	os.WriteFile("../web/artifacts/upgrade/checkout-migration-audit.json", b, 0600)
	fmt.Println(string(b))
}
