package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"digital-card-platform/backend/internal/admin"
	userauth "digital-card-platform/backend/internal/auth"
	"digital-card-platform/backend/internal/cards"
	"digital-card-platform/backend/internal/currency"
	"digital-card-platform/backend/internal/customer"
	"digital-card-platform/backend/internal/database"
	appmiddleware "digital-card-platform/backend/internal/middleware"
	"digital-card-platform/backend/internal/payments"
	"digital-card-platform/backend/internal/website"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
)

func main() {
	appEnv := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if appEnv == "" || appEnv == "local" || appEnv == "development" {
		_ = godotenv.Load()
	}
	appEnv = strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if appEnv == "production" {
		if err := validateProductionConfig(); err != nil {
			log.Fatal(err)
		}
	}

	if err := payments.ValidateConfig(); err != nil {
		log.Fatalf("Payment configuration invalid: %v", err)
	}

	pg, err := database.ConnectPostgres()
	if err != nil {
		log.Fatalf("PostgreSQL connection failed: %v", err)
	}
	defer pg.Close()

	rc, err := database.ConnectRedis()
	if err != nil {
		log.Fatalf("Redis connection failed: %v", err)
	}
	defer rc.Close()

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	r := chi.NewRouter()
	r.Use(appmiddleware.CORS)

	r.Use(
		chimiddleware.RequestID,
		chimiddleware.RealIP,
		chimiddleware.Logger,
		chimiddleware.Recoverer,
	)

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
			next.ServeHTTP(w, req)
		})
	})

	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		postgresStatus := "connected"
		redisStatus := "connected"
		if err := pg.Ping(ctx); err != nil {
			postgresStatus = "disconnected"
		}
		if err := rc.Ping(ctx).Err(); err != nil {
			redisStatus = "disconnected"
		}
		status := "ok"
		if postgresStatus != "connected" || redisStatus != "connected" {
			status = "unhealthy"
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":     status,
			"service":    "digital-card-api",
			"postgresql": postgresStatus,
			"redis":      redisStatus,
		})
	})

	// Public QR redirect and scan tracking endpoint.
	qrCodes := &admin.QRCodeHandler{DB: pg}
	r.Get("/q/{token}", qrCodes.Redirect)
	nfcTags := &admin.NFCHandler{DB: pg}
	r.Get("/n/{token}", nfcTags.Redirect)

	auth := &admin.AuthHandler{
		DB: pg,
	}

	dash := &admin.DashboardHandler{
		DB: pg,
	}

	analytics := &admin.AnalyticsHandler{DB: pg}
	plansBilling := &admin.PlansBillingHandler{DB: pg}
	paymentHandler := &payments.Handler{DB: pg}
	otpService := &userauth.OTPService{DB: pg, Sender: userauth.ZeptoMailSMTP{}}
	userAuth := &userauth.UserAuthHandler{DB: pg, OTP: otpService}
	adminOTP := &userauth.AdminOTPHandler{DB: pg, OTP: otpService}
	userDashboard := &customer.Handler{DB: pg}
	userCards := &cards.UserHandler{DB: pg, EmailSender: userauth.ZeptoMailSMTP{}}
	publicCards := &cards.PublicHandler{DB: pg}
	security := &admin.SecuritySettingsHandler{DB: pg}
	platformSettings := &admin.PlatformSettingsHandler{DB: pg}
	administration := &admin.AdministrationHandler{DB: pg}
	notifications := &admin.NotificationHandler{DB: pg}

	resources := &admin.ResourceHandler{DB: pg}

	ch := &cards.AdminHandler{
		DB: pg,
	}

	wh := &website.Handler{
		DB: pg,
	}

	fh := &website.FeatureHandler{
		DB: pg,
	}

	th := &website.TemplateHandler{
		DB: pg,
	}

	content := &website.ContentHandler{DB: pg}
	publicContent := &website.PublicContentHandler{DB: pg}
	publicTemplates := &website.PublicTemplateHandler{DB: pg}

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/public/currency/convert", (currency.Handler{Provider: currency.Frankfurter{Client: &http.Client{Timeout: 5 * time.Second}}}).Convert)
		r.Get("/public/currencies", (currency.Handler{}).List)
		r.Get("/public/billing/currencies", paymentHandler.CurrencyCatalog)
		r.Get("/public/payment-provider", paymentHandler.Provider)
		r.Post("/admin/auth/login", auth.Login)
		r.Post("/admin/auth/forgot-password", adminOTP.ForgotPassword)
		r.Post("/admin/auth/verify-otp", adminOTP.VerifyOTP)
		r.Post("/admin/auth/resend-otp", adminOTP.ResendOTP)
		r.Post("/admin/auth/reset-password", adminOTP.ResetPassword)
		r.Post("/auth/register", userAuth.Register)
		r.Post("/auth/login", userAuth.Login)
		r.Post("/auth/verify-otp", userAuth.VerifyOTP)
		r.Post("/auth/resend-otp", userAuth.ResendOTP)
		r.Post("/auth/forgot-password", userAuth.ForgotPassword)
		r.Post("/auth/reset-password", userAuth.ResetPassword)
		r.Post("/webhooks/razorpay", paymentHandler.Webhook)
		r.Get("/website/templates", publicTemplates.List)
		r.Get("/website/pricing", content.PublicPricing)
		r.Get("/public/cards/{slug}", publicCards.Get)
		r.Get("/public/cards/{slug}/qr", publicCards.QR)
		r.Get("/public/cards/{slug}/vcard", publicCards.VCard)
		r.Post("/public/cards/{slug}/events", publicCards.Event)
		r.Route("/user", func(r chi.Router) {
			r.Use(appmiddleware.UserAuth, appmiddleware.ActiveCustomerAccount(pg), userDashboard.Active)
			r.Get("/me", userDashboard.Me)
			registerEntitlementsRoute(r, userDashboard)
			r.Put("/profile", userDashboard.UpdateProfile)
			r.Post("/change-password", userDashboard.ChangePassword)
			r.Get("/dashboard", userDashboard.Dashboard)
			r.Get("/analytics", userDashboard.Analytics)
			r.Get("/cards", userCards.List)
			r.Get("/cards/{id}", userCards.Get)
			r.Post("/cards", userCards.Create)
			r.Put("/cards/{id}", userCards.Update)
			r.Patch("/cards/{id}/status", userCards.Status)
			r.Post("/cards/{id}/qr", userCards.CreateQR)
			r.Post("/cards/{id}/share/email", userCards.ShareEmail)
			r.Get("/contacts", userDashboard.Resources("contacts"))
			r.Get("/qr-codes", userDashboard.Resources("qr-codes"))
			r.Get("/nfc", userDashboard.NFCList)
			r.Post("/nfc", userDashboard.NFCCreate)
			r.Put("/nfc/{id}", userDashboard.NFCUpdate)
			r.Delete("/nfc/{id}", userDashboard.NFCArchive)
		})
		r.Group(func(r chi.Router) {
			r.Use(appmiddleware.UserAuth)
			r.Use(appmiddleware.ActiveCustomerAccount(pg))
			r.Get("/billing/provider", paymentHandler.Provider)
			r.Post("/billing/free/activate", paymentHandler.ActivateFreePlan)
			r.Post("/billing/checkout/order", paymentHandler.CreateOrder)
			r.Post("/billing/checkout/verify", paymentHandler.VerifyCheckout)
			r.Post("/billing/mock/order", paymentHandler.MockOrder)
			r.Post("/billing/mock/verify", paymentHandler.MockVerify)
			r.Get("/billing/history", paymentHandler.UserBilling)
			r.Get("/billing/history/{id}", paymentHandler.UserPaymentDetails)
		})
		r.Get("/public/plans", plansBilling.PublicPlans)
		r.Get("/public/website/home", wh.GetHome)
		r.Get("/public/website/features", fh.PublicList)
		r.Get("/public/website/templates", publicContent.Section("templates"))
		r.Get("/public/website/pricing-content", publicContent.Section("pricing"))
		r.Get("/public/website/business", publicContent.Section("business"))
		r.Get("/public/website/contact", publicContent.Section("contact"))
		r.Get("/public/website/header", publicContent.Section("header"))
		r.Get("/public/website/footer", publicContent.Section("footer"))
		r.Get("/public/website/seo", publicContent.Section("seo"))
		r.Post("/public/contact", publicContent.SubmitContact)
		r.Post("/website/contact", publicContent.SubmitContact)

		r.Route("/admin", func(r chi.Router) {
			r.Use(appmiddleware.AdminAuth)
			r.Use(appmiddleware.AdminAuthorization(pg))
			r.Get("/auth/me", auth.Me)
			r.Get("/administration/summary", administration.Summary)
			r.Get("/administration/users", administration.Users)
			r.Post("/administration/users", administration.Users)
			r.Get("/administration/users/{id}", administration.User)
			r.Put("/administration/users/{id}", administration.User)
			r.Patch("/administration/users/{id}", administration.User)
			r.Delete("/administration/users/{id}", administration.User)
			r.Get("/administration/user-types", administration.UserTypes)
			r.Post("/administration/user-types", administration.UserTypes)
			r.Get("/administration/user-types/{id}", administration.UserType)
			r.Put("/administration/user-types/{id}", administration.UserType)
			r.Patch("/administration/user-types/{id}", administration.UserType)
			r.Get("/administration/roles", administration.Roles)
			r.Post("/administration/roles", administration.Roles)
			r.Get("/administration/roles/{id}/permissions", administration.RolePermissions)
			r.Put("/administration/roles/{id}/permissions", administration.RolePermissions)
			r.Get("/administration/roles/{id}", administration.Role)
			r.Put("/administration/roles/{id}", administration.Role)
			r.Patch("/administration/roles/{id}", administration.Role)
			r.Delete("/administration/roles/{id}", administration.DeleteRole)
			r.Get("/administration/permissions", administration.Permissions)

			// Dashboard
			r.Get("/dashboard", dash.Get)
			r.Get("/notifications", notifications.List)
			r.Post("/notifications", notifications.Create)
			r.Get("/billing/summary", plansBilling.Summary)
			r.Get("/payments/summary", paymentHandler.AdminPaymentSummary)
			r.Get("/payments", paymentHandler.AdminPayments)
			r.Get("/payments/{id}", paymentHandler.AdminPaymentDetails)
			r.Patch("/payments/{id}/status", paymentHandler.CancelAdminPayment)
			r.Delete("/payments/{id}", paymentHandler.DeleteAdminPayment)
			r.Get("/transactions", paymentHandler.AdminTransactions)
			r.Get("/transactions/{id}", paymentHandler.AdminTransactionDetails)
			r.Get("/security/admin-users", security.AdminUsers)
			r.Get("/security/admin-users/{id}", security.AdminUser)
			r.Put("/security/admin-users/{id}", security.UpdateAdminUser)
			r.Patch("/security/admin-users/{id}/status", security.AdminUserStatus)
			r.Post("/security/admin-users/{id}/reset-password", security.ResetAdminPassword)
			r.Get("/security/settings", security.SecuritySettings)
			r.Put("/security/settings", security.SaveSecuritySettings)
			r.Get("/security/audit-log", security.AuditLog)
			r.Get("/settings", platformSettings.Get)
			r.Put("/settings", platformSettings.Save)
			r.Get("/settings/billing-global", paymentHandler.AdminGlobalBilling)
			r.Put("/settings/billing-global", paymentHandler.AdminGlobalBilling)
			r.Get("/plans/{id}/billing-prices", paymentHandler.PlanPrices)
			r.Put("/plans/{id}/billing-prices", paymentHandler.PlanPrices)
			r.Get("/settings/payment-currencies", paymentHandler.AdminCurrencySettings)
			r.Put("/settings/payment-currencies", paymentHandler.AdminCurrencySettings)
			r.Get("/settings/countries", platformSettings.Countries)
			r.Get("/settings/states", platformSettings.States)
			r.Get("/settings/languages", platformSettings.Languages)
			r.Get("/plans", plansBilling.ListPlans)
			r.Get("/plans/feature-registry", plansBilling.FeatureRegistry)
			r.Post("/plans", plansBilling.CreatePlan)
			r.Get("/plans/{id}/entitlements", plansBilling.GetPlanEntitlements)
			r.Put("/plans/{id}/entitlements", plansBilling.SavePlanEntitlements)
			r.Get("/plans/{id}", plansBilling.GetPlan)
			r.Put("/plans/{id}", plansBilling.UpdatePlan)
			r.Delete("/plans/{id}", plansBilling.DeletePlan)
			r.Patch("/plans/{id}/status", plansBilling.PlanStatus)
			r.Get("/subscriptions", plansBilling.ListSubscriptions)
			r.Post("/subscriptions", plansBilling.CreateSubscription)
			r.Get("/subscriptions/{id}", plansBilling.GetSubscription)
			r.Put("/subscriptions/{id}", plansBilling.UpdateSubscription)
			r.Patch("/subscriptions/{id}/status", plansBilling.SubscriptionStatus)
			r.Get("/analytics/summary", analytics.Summary)
			r.Get("/analytics/timeseries", analytics.Timeseries)
			r.Get("/analytics/top-cards", analytics.TopCards)
			r.Get("/analytics/recent", analytics.Recent)
			r.Get("/qr-codes/summary", qrCodes.Summary)
			r.Get("/qr-codes", qrCodes.List)
			r.Post("/qr-codes", qrCodes.Create)
			r.Get("/qr-codes/{id}/scans", qrCodes.Scans)
			r.Get("/qr-codes/{id}", qrCodes.Get)
			r.Put("/qr-codes/{id}", qrCodes.Update)
			r.Delete("/qr-codes/{id}", qrCodes.Delete)
			r.Get("/nfc/summary", nfcTags.Summary)
			r.Get("/nfc", nfcTags.List)
			r.Post("/nfc", nfcTags.Create)
			r.Get("/nfc/{id}/taps", nfcTags.Taps)
			r.Put("/nfc/{id}/assign", nfcTags.Assign)
			r.Put("/nfc/{id}/unassign", nfcTags.Unassign)
			r.Get("/nfc/{id}", nfcTags.Get)
			r.Put("/nfc/{id}", nfcTags.Update)
			r.Delete("/nfc/{id}", nfcTags.Delete)
			r.Get("/users", resources.ListUsers)
			r.Get("/users/{id}", resources.GetUser)
			r.Post("/users", resources.CreateUser)
			r.Put("/users/{id}", resources.UpdateUser)
			r.Delete("/users/{id}", resources.DeleteUser)
			r.Get("/organizations", resources.ListOrganizations)
			r.Get("/organizations/{id}", resources.GetOrganization)
			r.Post("/organizations", resources.CreateOrganization)
			r.Put("/organizations/{id}", resources.UpdateOrganization)
			r.Delete("/organizations/{id}", resources.DeleteOrganization)
			r.Get("/contacts", resources.ListContacts)
			r.Post("/contacts", resources.CreateContact)
			r.Get("/contacts/{id}", resources.GetContact)
			r.Put("/contacts/{id}", resources.UpdateContact)
			r.Delete("/contacts/{id}", resources.DeleteContact)

			// Digital Cards
			r.Get("/cards", ch.List)
			r.Post("/cards", ch.Create)
			r.Get("/cards/{id}", ch.Get)
			r.Put("/cards/{id}", ch.Update)
			r.Patch("/cards/{id}/status", ch.Status)
			r.Delete("/cards/{id}", ch.Delete)
			r.Put("/cards/{id}/template", ch.SetTemplate)

			// The sidebar catalogue and Website Templates share website_templates.
			r.Get("/templates", th.List)
			r.Get("/templates/{id}", th.Get)
			r.Get("/templates/{id}/cards", th.Cards)

			// Website Home
			r.Get("/website/home", wh.GetHome)
			r.Put("/website/home", wh.UpdateHome)
			r.Post("/website/home/reset", wh.ResetHome)

			// Website Features
			r.Get("/website/features", fh.List)
			r.Post("/website/features", fh.Create)
			r.Put("/website/features/{id}", fh.Update)
			r.Put("/website/features/order", fh.SetOrder)
			r.Delete("/website/features/{id}", fh.Delete)

			// Website Templates
			r.Get("/website/templates", th.List)
			r.Post("/website/templates", th.Create)
			r.Get("/website/templates/{id}", th.Get)
			r.Put("/website/templates/{id}", th.Update)
			r.Delete("/website/templates/{id}", th.Delete)
			r.Get("/website/sections/{section}", content.GetSection)
			r.Put("/website/sections/{section}", content.SaveSection)
			r.Post("/website/sections/features/reset", content.ResetFeaturesPageContent)
			r.Delete("/website/sections/features", content.DeleteFeaturesPageContent)
			// Editable website modules use the same JSONB-backed section store.
			// Keep named routes convenient for dashboard clients while retaining
			// the generic sections API for additional sections.
			// /website/features is the feature-item list route above. Page-level
			// Features CMS settings remain available at /website/sections/features.
			for _, section := range namedWebsiteSectionAliases() {
				r.Get("/website/"+section, content.GetNamedSection(section))
				r.Put("/website/"+section, content.SaveNamedSection(section))
			}
			r.Get("/website/pricing", content.ListPricing)
			r.Post("/website/pricing", content.SavePricing)
			r.Put("/website/pricing/{id}", content.SavePricing)
			r.Patch("/website/pricing/{id}/status", content.SetPricingStatus)
			r.Delete("/website/pricing/{id}", content.DeletePricing)
			r.Get("/website/contact/enquiries", publicContent.ListEnquiries)
			r.Get("/website/contact/enquiries/{id}", publicContent.GetEnquiry)
			r.Patch("/website/contact/enquiries/{id}/status", publicContent.SetEnquiryStatus)
			r.Put("/website/contact/enquiries/{id}/note", publicContent.SetEnquiryNote)
			r.Delete("/website/contact/enquiries/{id}", publicContent.DeleteEnquiry)
		})
	})

	addr := ":" + port
	server := &http.Server{Addr: addr, Handler: r, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("API listening on %s (APP_ENV=%s)", addr, appEnv)
	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.ListenAndServe() }()
	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("API server failed: %v", err)
		}
	case <-shutdownCtx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Graceful shutdown failed: %v", err)
			_ = server.Close()
		}
		if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("API server stopped with error: %v", err)
		}
	}
}

// registerEntitlementsRoute keeps the customer entitlement URL in one place;
// callers mount it inside the existing authenticated /user route group.
func registerEntitlementsRoute(r chi.Router, handler *customer.Handler) {
	r.Get("/me/entitlements", handler.Entitlements)
}

// namedWebsiteSectionAliases excludes features because /website/features is
// the feature-item CRUD list route. Feature page settings use
// /website/sections/features instead.
func namedWebsiteSectionAliases() []string {
	return []string{"templates", "pricing", "business", "contact", "header", "footer", "seo"}
}

func validateProductionConfig() error {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	lowerSecret := strings.ToLower(secret)
	if len(secret) < 32 || strings.Contains(lowerSecret, "replace") || strings.Contains(lowerSecret, "changeme") || strings.Contains(lowerSecret, "change-me") || strings.Contains(lowerSecret, "placeholder") || strings.Contains(lowerSecret, "your_") || strings.Contains(lowerSecret, "example") {
		return errors.New("JWT_SECRET must be configured with at least 32 non-placeholder characters in production")
	}
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("PAYMENT_PROVIDER")), "razorpay") {
		return errors.New("PAYMENT_PROVIDER must be razorpay in production; mock payment is local-only")
	}
	if err := payments.ValidateConfig(); err != nil {
		return err
	}
	if err := userauth.ValidateProductionSMTPConfig(); err != nil {
		return err
	}
	for _, key := range []string{"FRONTEND_URL", "PUBLIC_WEB_URL", "API_PUBLIC_URL"} {
		value := strings.TrimSpace(os.Getenv(key))
		u, err := url.Parse(value)
		lower := strings.ToLower(value)
		if err != nil || u.Scheme != "https" || u.Host == "" || (u.Path != "" && u.Path != "/") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Contains(lower, "your_domain") || strings.Contains(lower, "replace_with") {
			return errors.New(key + " must be a valid HTTPS origin in production")
		}
	}
	if value := strings.TrimSpace(os.Getenv("APP_PORT")); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			return errors.New("APP_PORT must be a valid TCP port")
		}
	}
	return nil
}
