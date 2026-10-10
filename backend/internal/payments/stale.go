package payments

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

// Local unpaid state can lag the gateway. Never invalidate an order that the
// provider reports as paid or attempted; reconcile it first.
func guardStaleOrders(ctx context.Context, tx pgx.Tx, gateway Gateway, userID, planID int64, code, amount, period, country string) error {
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(candidate.provider_order_id),'[]'::jsonb) FROM (SELECT provider_order_id FROM payments WHERE user_id=$1 AND provider='razorpay' AND status IN ('created','pending') AND subscription_id IS NULL AND (plan_id<>$2 OR UPPER(currency)<>$3 OR amount<>$4::numeric OR COALESCE(billing_period,$5)<>$5 OR checkout_country<>$6 OR created_at<NOW()-INTERVAL '30 minutes') FOR UPDATE) candidate`, userID, planID, code, amount, period, country).Scan(&raw)
	if err != nil {
		return errors.New("Could not inspect stale orders")
	}
	var orders []string
	if json.Unmarshal(raw, &orders) != nil {
		return errors.New("Could not inspect stale orders")
	}
	for _, id := range orders {
		order, err := gateway.FetchOrder(ctx, id)
		if err != nil || order.ID != id {
			return errors.New("Could not confirm the previous checkout with the payment provider. Please retry or contact support")
		}
		if order.Status != "created" {
			return errors.New("A previous checkout needs payment reconciliation before it can be replaced. Refresh billing or contact support")
		}
	}
	return nil
}
func recordCancelledCapture(ctx context.Context, tx pgx.Tx, id int64, paymentID, method string) error {
	_, err := tx.Exec(ctx, `UPDATE payments SET status='paid',provider_payment_id=$1,payment_method=NULLIF($2,''),paid_at=COALESCE(paid_at,NOW()),error_code='RECONCILIATION_REQUIRED',error_description='Verified capture after checkout cancellation; review refund or fulfillment manually',updated_at=NOW() WHERE id=$3 AND status='cancelled'`, paymentID, safeMethod(method), id)
	return err
}
