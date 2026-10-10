// Real provider test-mode order creation probes; never creates/captures a payment.
package main

import (
	"context"
	"digital-card-platform/backend/internal/currency"
	"digital-card-platform/backend/internal/razorpay"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"os"
	"strings"
	"time"
)

func main() {
	_ = godotenv.Load()
	if os.Getenv("APP_ENV") != "local" || os.Getenv("RAZORPAY_MODE") != "test" || os.Getenv("PAYMENT_PROVIDER") != "razorpay" || !strings.HasPrefix(os.Getenv("RAZORPAY_KEY_ID"), "rzp_test_") {
		panic("Provider probes require local Razorpay test mode")
	}
	c := razorpay.Client{KeyID: os.Getenv("RAZORPAY_KEY_ID"), KeySecret: os.Getenv("RAZORPAY_KEY_SECRET")}
	results := []map[string]any{}
	for _, code := range currency.Codes() {
		minor, err := razorpay.CheckoutAmountToMinor("25.00", code)
		if err != nil {
			panic("Invalid probe amount")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		order, err := c.CreateOrder(ctx, minor, code, fmt.Sprintf("qa-%s-%d", code, time.Now().UnixNano()))
		cancel()
		row := map[string]any{"currency": code, "amount_major": "25.00", "amount_minor": minor, "success": err == nil}
		if err != nil {
			row["failure_kind"] = razorpay.FailureKind(err)
			row["safe_diagnostic"] = err.Error()
		} else {
			row["order_id"] = order.ID
			row["returned_amount"] = order.Amount
			row["returned_currency"] = order.Currency
			row["success"] = order.ID != "" && order.Amount == minor && order.Currency == code
		}
		results = append(results, row)
	}
	b, _ := json.MarshalIndent(results, "", "  ")
	if os.WriteFile("../web/artifacts/upgrade/provider-currencies.json", b, 0600) != nil {
		panic("Could not record probe")
	}
	fmt.Println(string(b))
}
