package middleware

import (
	"net/http"
	"testing"
)

func TestPaymentDeleteRequiresBillingManage(t *testing.T) {
	if got := RequiredAdminPermission(http.MethodGet, "/api/v1/admin/payments"); got != "billing.view" {
		t.Fatalf("GET payments permission = %q, want billing.view", got)
	}
	if got := RequiredAdminPermission(http.MethodDelete, "/api/v1/admin/payments/12"); got != "billing.manage" {
		t.Fatalf("DELETE payment permission = %q, want billing.manage", got)
	}
	if got := RequiredAdminPermission(http.MethodPatch, "/api/v1/admin/payments/12/status"); got != "billing.manage" {
		t.Fatalf("PATCH payment status permission = %q, want billing.manage", got)
	}
	if got := RequiredAdminPermission(http.MethodPost, "/api/v1/admin/payments/12"); got != "" {
		t.Fatalf("unexpected permission for unsupported payment method: %q", got)
	}
}

func TestPlanManagementRequiresBillingPermissions(t *testing.T) {
	if got := RequiredAdminPermission(http.MethodGet, "/api/v1/admin/plans"); got != "billing.view" {
		t.Fatalf("GET plans permission = %q, want billing.view", got)
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		if got := RequiredAdminPermission(method, "/api/v1/admin/plans/12"); got != "billing.manage" {
			t.Errorf("%s plans permission = %q, want billing.manage", method, got)
		}
	}
}

func TestWebsiteCMSRequiresWebsitePermissions(t *testing.T) {
	if got := RequiredAdminPermission(http.MethodGet, "/api/v1/admin/website/home"); got != "website.view" {
		t.Fatalf("GET website content permission = %q, want website.view", got)
	}
	if got := RequiredAdminPermission(http.MethodPut, "/api/v1/admin/website/sections/footer"); got != "website.manage" {
		t.Fatalf("PUT website content permission = %q, want website.manage", got)
	}
	if got := RequiredAdminPermission(http.MethodPut, "/api/v1/admin/website/features/order"); got != "website.manage" {
		t.Fatalf("PUT feature order permission = %q, want website.manage", got)
	}
}
