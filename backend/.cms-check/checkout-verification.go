// Verify genuine Razorpay TEST capture and replay its signed webhook locally.
package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"digital-card-platform/backend/internal/razorpay"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	_ = godotenv.Load()
	if os.Getenv("APP_ENV") != "local" || os.Getenv("RAZORPAY_MODE") != "test" || !strings.HasPrefix(os.Getenv("RAZORPAY_KEY_ID"), "rzp_test_") {
		panic("TEST local only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	cfg, e := pgx.ParseConfig(os.Getenv("DATABASE_URL"))
	if e != nil || (cfg.Host != "localhost" && cfg.Host != "127.0.0.1") {
		panic("Local DB only")
	}
	db, e := pgx.ConnectConfig(ctx, cfg)
	if e != nil {
		panic("Local database unavailable")
	}
	defer db.Close(ctx)
	wanted, identity, output := "captured", "../web/.feature-qa/upgrade-customer.json", "../web/artifacts/upgrade/checkout-verification.json"
	if len(os.Args) > 1 && os.Args[1] == "failed" {
		wanted = "failed"
		identity = "../web/.feature-qa/trial-customer.json"
		output = "../web/artifacts/upgrade/checkout-failed-verification.json"
	}
	raw, e := os.ReadFile(identity)
	if e != nil {
		panic("QA identity missing")
	}
	var a struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
	}
	_ = json.Unmarshal(raw, &a)
	if a.ID < 1 || !strings.HasPrefix(a.Email, "upgrade-qa-") {
		panic("Invalid QA identity")
	}
	var payID, order, currency, price string
	var id int64
	if wanted == "captured" {
		e = db.QueryRow(ctx, `SELECT id,provider_payment_id,provider_order_id,currency,amount::text FROM payments WHERE user_id=$1 AND provider='razorpay' AND status='paid' ORDER BY paid_at DESC LIMIT 1`, a.ID).Scan(&id, &payID, &order, &currency, &price)
	} else {
		e = db.QueryRow(ctx, `SELECT id,provider_order_id,currency,amount::text FROM payments WHERE user_id=$1 AND provider='razorpay' AND status IN ('created','failed') ORDER BY created_at DESC LIMIT 1`, a.ID).Scan(&id, &order, &currency, &price)
	}
	if e != nil {
		panic("No genuine paid record")
	}
	c := razorpay.Client{KeyID: os.Getenv("RAZORPAY_KEY_ID"), KeySecret: os.Getenv("RAZORPAY_KEY_SECRET")}
	if wanted == "failed" {
		req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.razorpay.com/v1/orders/"+order+"/payments", nil)
		req.SetBasicAuth(c.KeyID, c.KeySecret)
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			panic("Provider order unavailable")
		}
		var collection struct {
			Items []razorpay.Payment `json:"items"`
		}
		err = json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&collection)
		r.Body.Close()
		if r.StatusCode != 200 || err != nil {
			panic("Provider order lookup failed")
		}
		for _, item := range collection.Items {
			if item.Status == "failed" {
				payID = item.ID
			}
		}
		if payID == "" {
			panic("No genuine failed provider payment")
		}
	}
	p, e := c.FetchPayment(ctx, payID)
	if e != nil {
		panic("Provider payment fetch failed")
	}
	minor, e := razorpay.AmountToMinor(price, currency)
	if e != nil || p.Status != wanted || p.OrderID != order || p.Currency != currency || p.Amount != minor {
		panic("Genuine payment did not match")
	}
	counts := func() []int {
		var sub, tx int
		if db.QueryRow(ctx, `SELECT COUNT(*) FROM subscriptions WHERE user_id=$1`, a.ID).Scan(&sub) != nil || db.QueryRow(ctx, `SELECT COUNT(*) FROM payment_transactions WHERE payment_id=$1`, id).Scan(&tx) != nil {
			panic("Count query failed")
		}
		return []int{sub, tx}
	}
	before := counts()
	body, _ := json.Marshal(map[string]any{"event": "payment." + wanted, "payload": map[string]any{"payment": map[string]any{"entity": p}}})
	secret := os.Getenv("RAZORPAY_WEBHOOK_SECRET")
	if secret == "" {
		panic("Webhook secret missing")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))
	statuses := []int{}
	responses := []string{}
	var middle []int
	for _, sig := range []string{"invalid", signature, signature} {
		req, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost:8080/api/v1/webhooks/razorpay", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Razorpay-Signature", sig)
		req.Header.Set("X-Razorpay-Event-Id", "qa-replay-"+p.ID)
		r, e := http.DefaultClient.Do(req)
		if e != nil {
			panic("Local webhook unavailable")
		}
		b, _ := io.ReadAll(io.LimitReader(r.Body, 2048))
		r.Body.Close()
		statuses = append(statuses, r.StatusCode)
		responses = append(responses, string(b))
		if len(statuses) == 2 {
			middle = counts()
		}
	}
	after := counts()
	if statuses[0] != 400 || statuses[1] != 200 || statuses[2] != 200 || before[0] != after[0] || middle[1] != after[1] {
		panic("Signature/idempotence check failed")
	}
	result := map[string]any{"genuine_provider_payment": p, "verified_amount_major": price, "webhook_delivery": "local signed replay after genuine provider fetch; not remote delivery", "statuses": statuses, "responses": responses, "counts_before": before, "counts_after": after}
	b, _ := json.MarshalIndent(result, "", "  ")
	if wanted == "failed" {
		var status string
		var paid int
		var trial bool
		if db.QueryRow(ctx, `SELECT status FROM payments WHERE id=$1`, id).Scan(&status) != nil || status != "failed" {
			panic("Failure record missing")
		}
		if db.QueryRow(ctx, `SELECT COUNT(*) FROM payments WHERE user_id=$1 AND status='paid'`, a.ID).Scan(&paid) != nil || paid != 0 {
			panic("Failure activated paid payment")
		}
		if db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM subscriptions WHERE user_id=$1 AND status='trial' AND is_trial)`, a.ID).Scan(&trial) != nil || !trial {
			panic("Failure changed trial")
		}
	}
	os.WriteFile(output, b, 0600)
	fmt.Println(string(b))
}
