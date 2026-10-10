// paymentqa performs read-only billing schema and configuration checks.
package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"os"
	"strings"
	"time"
)

func main() {
	_ = godotenv.Load()
	for _, key := range []string{"RAZORPAY_KEY_ID", "RAZORPAY_KEY_SECRET", "RAZORPAY_WEBHOOK_SECRET"} {
		v := strings.TrimSpace(os.Getenv(key))
		low := strings.ToLower(v)
		fmt.Printf("%s: set=%t placeholder=%t\n", key, v != "", strings.Contains(low, "replace_with") || strings.Contains(low, "your_") || strings.Contains(low, "placeholder"))
	}
	fmt.Printf("Razorpay test mode=%t; Razorpay provider selected=%t\n", os.Getenv("RAZORPAY_MODE") == "test", os.Getenv("PAYMENT_PROVIDER") == "razorpay")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Println("Database unavailable; connection details suppressed")
		os.Exit(1)
	}
	defer conn.Close(ctx)
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		panic("Could not begin read-only QA")
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT table_name,column_name,data_type FROM information_schema.columns WHERE table_schema='public' AND table_name IN ('website_pricing_plans','payments','payment_transactions','processed_payment_webhook_events','subscriptions','users') ORDER BY table_name,ordinal_position`)
	if err != nil {
		panic("Schema inspection failed")
	}
	for rows.Next() {
		var table, col, typ string
		rows.Scan(&table, &col, &typ)
		fmt.Printf("%s.%s %s\n", table, col, typ)
	}
	rows.Close()
	for _, q := range []string{
		`SELECT id,user_id,plan_id,provider,provider_order_id,provider_payment_id,amount,currency,status,payment_method,paid_at,created_at FROM payments ORDER BY id DESC LIMIT 10`,
		`SELECT id,user_id,plan_id,status,started_at,ends_at,renews_at FROM subscriptions WHERE status='active' ORDER BY id DESC LIMIT 10`,
		`SELECT user_id,COUNT(*) FROM subscriptions WHERE status='active' AND (ends_at IS NULL OR ends_at>NOW()) GROUP BY user_id HAVING COUNT(*)>1`,
	} {
		rows, err := tx.Query(ctx, q)
		if err != nil {
			fmt.Println("QA query failed; check schema")
			continue
		}
		for rows.Next() {
			v, e := rows.Values()
			if e == nil {
				fmt.Println(v)
			}
		}
		rows.Close()
	}
}
