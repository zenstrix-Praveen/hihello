package payments

import (
	"context"
	"digital-card-platform/backend/internal/razorpay"
)

// Gateway operates in integer ISO minor units. Provider adapters must enforce
// their own additional amount rules, and declare capabilities separately.
type GatewayOrder struct {
	ID               string
	Amount           int64
	Currency, Status string
}
type GatewayPayment struct {
	ID, OrderID              string
	Amount                   int64
	Currency, Status, Method string
}
type Gateway interface {
	CreateOrder(context.Context, int64, string, string) (GatewayOrder, error)
	FetchOrder(context.Context, string) (GatewayOrder, error)
	FetchPayment(context.Context, string) (GatewayPayment, error)
	VerifyCheckout(string, string, string) bool
	VerifyCallback([]byte, string) bool
}
type razorpayGateway struct {
	client        razorpay.Client
	webhookSecret string
}

func (g razorpayGateway) CreateOrder(ctx context.Context, amount int64, code, receipt string) (GatewayOrder, error) {
	o, e := g.client.CreateOrder(ctx, amount, code, receipt)
	return GatewayOrder{o.ID, o.Amount, o.Currency, o.Status}, e
}
func (g razorpayGateway) FetchOrder(ctx context.Context, id string) (GatewayOrder, error) {
	o, e := g.client.FetchOrder(ctx, id)
	return GatewayOrder{o.ID, o.Amount, o.Currency, o.Status}, e
}
func (g razorpayGateway) FetchPayment(ctx context.Context, id string) (GatewayPayment, error) {
	p, e := g.client.FetchPayment(ctx, id)
	return GatewayPayment{p.ID, p.OrderID, p.Amount, p.Currency, p.Status, p.Method}, e
}
func (g razorpayGateway) VerifyCheckout(order, payment, signature string) bool {
	return razorpay.VerifyCheckoutSignature(order, payment, signature, g.client.KeySecret)
}
func (g razorpayGateway) VerifyCallback(body []byte, signature string) bool {
	return razorpay.VerifyWebhookSignature(body, signature, g.webhookSecret)
}

var _ Gateway = razorpayGateway{}

// Stripe, PayPal and regional providers can implement Gateway; no adapter or
// routing activation is provided until explicitly configured and approved.
