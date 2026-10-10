// Create a clearly marked local QAR QA plan without editing the customer's INR plan.
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
	if os.Getenv("APP_ENV") != "local" || os.Getenv("RAZORPAY_MODE") != "test" {
		panic("Local TEST only")
	}
	cfg, e := pgx.ParseConfig(os.Getenv("DATABASE_URL"))
	if e != nil || (cfg.Host != "localhost" && cfg.Host != "127.0.0.1") {
		panic("Local database only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, e := pgx.ConnectConfig(ctx, cfg)
	if e != nil {
		panic("Local DB unavailable")
	}
	defer db.Close(ctx)
	const file = "../web/.feature-qa/qar-plan.json"
	if len(os.Args) > 1 && os.Args[1] == "disable" {
		b, e := os.ReadFile(file)
		if e != nil {
			panic("QA plan identity missing")
		}
		var a struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal(b, &a)
		tag, e := db.Exec(ctx, `UPDATE website_pricing_plans SET is_enabled=FALSE WHERE id=$1 AND name='QA Professional QAR checkout'`, a.ID)
		if e != nil || tag.RowsAffected() != 1 {
			panic("QA plan identity mismatch")
		}
		fmt.Println("Disabled QA QAR plan; billing history retained")
		return
	}
	var id int64
	e = db.QueryRow(ctx, `INSERT INTO website_pricing_plans(code,name,description,price,currency,billing_period,is_enabled,pricing_type,trial_mode,trial_days,billing_type) SELECT 'qa-qar-checkout-'||extract(epoch FROM NOW())::text,'QA Professional QAR checkout','Temporary localhost TEST checkout verification',price,'QAR',billing_period,TRUE,pricing_type,'disabled',0,billing_type FROM website_pricing_plans WHERE id=10 RETURNING id`).Scan(&id)
	if e != nil {
		fmt.Println(e.Error())
		panic("Could not create QA plan")
	}
	b, _ := json.Marshal(map[string]any{"id": id})
	os.WriteFile(file, b, 0600)
	fmt.Println(string(b))
}
