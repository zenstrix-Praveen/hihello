// Applies the additive trial-history migration to the configured local DB.
package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"os"
	"time"
)

func main() {
	_ = godotenv.Load()
	if os.Getenv("APP_ENV") != "local" {
		panic("This QA migration runner is restricted to APP_ENV=local")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		panic("Local database connection failed")
	}
	defer db.Close(ctx)
	sql, err := os.ReadFile("migrations/000032_subscription_trial_history.sql")
	if err != nil {
		panic(err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		panic("Could not begin migration")
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, string(sql)); err != nil {
		panic("Trial-history migration failed")
	}
	if err = tx.Commit(ctx); err != nil {
		panic("Migration commit failed")
	}
	fmt.Println("Applied additive trial-history migration")
	var legacyFreeMarkers int
	if err = db.QueryRow(ctx, `SELECT COUNT(*) FROM subscriptions s JOIN website_pricing_plans p ON p.id=s.plan_id WHERE s.is_trial AND (p.price=0 OR p.pricing_type='free')`).Scan(&legacyFreeMarkers); err != nil {
		panic("Could not audit free plan markers")
	}
	fmt.Printf("Free-plan subscriptions marked as prior trials: %d\n", legacyFreeMarkers)
	rows, err := db.Query(ctx, `SELECT s.id,s.user_id,s.plan_id,s.status,s.amount::text,s.started_at,s.updated_at,p.name,p.price::text,(SELECT COUNT(*) FROM payments x WHERE x.user_id=s.user_id AND x.status='paid') FROM subscriptions s JOIN website_pricing_plans p ON p.id=s.plan_id WHERE s.is_trial AND (p.price=0 OR p.pricing_type='free') ORDER BY s.id`)
	if err != nil {
		panic("Audit query failed")
	}
	defer rows.Close()
	for rows.Next() {
		var id, user, plan int64
		var status, amount, name, price string
		var start, update time.Time
		var paid int
		if rows.Scan(&id, &user, &plan, &status, &amount, &start, &update, &name, &price, &paid) != nil {
			panic("Audit row invalid")
		}
		fmt.Printf("Free marker subscription=%d user=%d plan=%d status=%s amount=%s started=%s updated=%s plan_name=%s current_price=%s paid_records=%d\n", id, user, plan, status, amount, start.Format(time.RFC3339), update.Format(time.RFC3339), name, price, paid)
	}
}
