// billing-inspect reads only non-secret billing configuration and aggregate checkout state.
package main

import (
	"context"
	"digital-card-platform/backend/internal/payments"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"os"
	"time"
)

func main() {
	_ = godotenv.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Println("Database unavailable")
		os.Exit(1)
	}
	defer conn.Close(ctx)
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		fmt.Println("Read-only transaction unavailable")
		os.Exit(1)
	}
	defer tx.Rollback(ctx)
	fmt.Printf("payment_provider=%s mode=%s environment_currencies=%v\n", os.Getenv("PAYMENT_PROVIDER"), os.Getenv("RAZORPAY_MODE"), payments.SupportedCurrencies("razorpay"))
	rows, err := tx.Query(ctx, `SELECT setting_key,setting_value FROM platform_settings WHERE setting_key IN ('razorpay_supported_currencies','razorpay_merchant_currencies_verified','razorpay_currency_confirmation_mode','billing_global_test','billing_global_live') ORDER BY setting_key`)
	if err == nil {
		for rows.Next() {
			var key, value string
			rows.Scan(&key, &value)
			var data any
			if json.Unmarshal([]byte(value), &data) == nil {
				safe, _ := json.Marshal(data)
				fmt.Printf("%s=%s\n", key, safe)
			}
		}
		rows.Close()
	}
	rows, err = tx.Query(ctx, `SELECT name,price::text,currency,billing_period FROM website_pricing_plans WHERE name ILIKE '%profession%' AND archived_at IS NULL`)
	if err == nil {
		for rows.Next() {
			var name, price, code, period string
			rows.Scan(&name, &price, &code, &period)
			fmt.Printf("plan=%s price=%s currency=%s period=%s\n", name, price, code, period)
		}
		rows.Close()
	}
	rows, err = tx.Query(ctx, `SELECT p.status,p.currency,p.amount::text,COUNT(*) FROM payments p JOIN website_pricing_plans plan ON plan.id=p.plan_id WHERE p.status IN ('created','pending','authorized') AND (p.currency<>plan.currency OR p.amount<>plan.price) GROUP BY p.status,p.currency,p.amount ORDER BY p.status,p.currency`)
	if err == nil {
		for rows.Next() {
			var status, code, amount string
			var count int
			rows.Scan(&status, &code, &amount, &count)
			fmt.Printf("incompatible_open_checkout status=%s currency=%s amount=%s count=%d\n", status, code, amount, count)
		}
		rows.Close()
	}
}
