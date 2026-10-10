package payments

import (
	"digital-card-platform/backend/internal/razorpay"
	"net/http"
)

func checkoutFailure(err error) (int, string) {
	switch razorpay.FailureKind(err) {
	case "network", "timeout":
		return http.StatusBadGateway, "Unable to connect to the payment provider. Please try again or contact support."
	case "authentication":
		return http.StatusServiceUnavailable, "The payment gateway credentials are not configured correctly. Please contact support."
	case "currency":
		return http.StatusUnprocessableEntity, "This currency is not enabled for the configured payment provider. Please contact support to confirm merchant currency availability or select another plan."
	case "unavailable":
		return http.StatusServiceUnavailable, "The payment provider is temporarily unavailable. Please retry shortly."
	default:
		return http.StatusBadGateway, "Payment could not be initialized. Please retry or contact support."
	}
}
