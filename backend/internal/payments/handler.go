package payments

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"digital-card-platform/backend/internal/currency"
	"digital-card-platform/backend/internal/middleware"
	"digital-card-platform/backend/internal/pricing"
	"digital-card-platform/backend/internal/razorpay"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type Database interface {
	Begin(context.Context) (pgx.Tx, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type Handler struct {
	DB           Database
	ProviderHTTP *http.Client
}

func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (h *Handler) client() (razorpay.Client, error) {
	if err := ValidateConfig(); err != nil {
		return razorpay.Client{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("PAYMENT_PROVIDER")), "razorpay") {
		return razorpay.Client{}, errors.New("Razorpay provider is not selected")
	}
	return razorpay.Client{KeyID: strings.TrimSpace(os.Getenv("RAZORPAY_KEY_ID")), KeySecret: strings.TrimSpace(os.Getenv("RAZORPAY_KEY_SECRET")), HTTP: h.ProviderHTTP}, nil
}

func requestUserID(r *http.Request) (int64, bool) {
	id, ok := r.Context().Value(middleware.UserIDKey).(int64)
	return id, ok && id > 0
}

func (h *Handler) configuredCurrencies(provider string) ([]string, bool, string) {
	if provider == "mock" {
		return SupportedCurrencies(provider), false, "local mock provider"
	}
	if provider != "razorpay" {
		return []string{}, false, "provider is not configured"
	}
	if h.DB != nil {
		var raw, verifiedRaw string
		err := h.DB.QueryRow(context.Background(), `SELECT setting_value FROM platform_settings WHERE setting_key='razorpay_supported_currencies'`).Scan(&raw)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return []string{}, false, "Merchant settings are unavailable"
		}
		if err == nil {
			var codes []string
			var verified bool
			_ = json.Unmarshal([]byte(raw), &codes)
			if h.DB.QueryRow(context.Background(), `SELECT setting_value FROM platform_settings WHERE setting_key='razorpay_merchant_currencies_verified'`).Scan(&verifiedRaw) == nil {
				_ = json.Unmarshal([]byte(verifiedRaw), &verified)
			}
			clean, cleanErr := normalizeConfiguredCurrencies(codes)
			var storedMode string
			modeErr := h.DB.QueryRow(context.Background(), `SELECT setting_value FROM platform_settings WHERE setting_key='razorpay_currency_confirmation_mode'`).Scan(&storedMode)
			var confirmedMode string
			_ = json.Unmarshal([]byte(storedMode), &confirmedMode)
			mode := strings.ToLower(strings.TrimSpace(os.Getenv("RAZORPAY_MODE")))
			scoped := modeErr == nil && confirmedMode == mode
			if mode == "test" && errors.Is(modeErr, pgx.ErrNoRows) {
				scoped = true
			}
			if cleanErr == nil && verified && scoped {
				return clean, true, "Admin Payment Settings (merchant currency confirmation recorded)"
			}
			return []string{}, false, "Admin Payment Settings (merchant currency confirmation required)"
		}
	}
	return SupportedCurrencies(provider), false, CurrencyConfigurationSource(provider)
}

func (h *Handler) checkoutCurrencySupported(provider, code string) bool {
	values, _, _ := h.configuredCurrencies(provider)
	return currencyInList(values, code)
}

func currencyInList(values []string, code string) bool {
	code = currency.Normalize(code)
	for _, allowed := range values {
		if code == allowed {
			return true
		}
	}
	return false
}

// AdminCurrencySettings exposes the existing provider's currency setup without
// exposing credentials. A currency can only be enabled after an administrator
// confirms it is active in the merchant account.
func (h *Handler) AdminCurrencySettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		codes, verified, source := h.configuredCurrencies("razorpay")
		jsonResponse(w, http.StatusOK, map[string]any{"data": map[string]any{"provider": "razorpay", "mode": strings.ToLower(strings.TrimSpace(os.Getenv("RAZORPAY_MODE"))), "provider_configured": ValidateConfig() == nil, "currencies": currency.Catalog(), "supported_currencies": codes, "published_supported_currencies": json.RawMessage(publishedCurrenciesJSON), "merchant_currencies_verified": verified, "currency_configuration_source": source}})
		return
	}
	if r.Method != http.MethodPut {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	var in struct {
		Currencies        []string `json:"currencies"`
		MerchantConfirmed bool     `json:"merchant_confirmed"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 16<<10))
	dec.DisallowUnknownFields()
	if dec.Decode(&in) != nil || dec.Decode(new(any)) != io.EOF {
		jsonResponse(w, 400, map[string]string{"error": "Invalid payment currency configuration"})
		return
	}
	codes, err := normalizeConfiguredCurrencies(in.Currencies)
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if len(codes) > 0 && !in.MerchantConfirmed {
		jsonResponse(w, 400, map[string]string{"error": "Confirm these currencies are enabled for the Razorpay merchant account before saving"})
		return
	}
	if h.DB == nil {
		jsonResponse(w, 503, map[string]string{"error": "Payment settings storage is unavailable"})
		return
	}
	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not save payment currency settings"})
		return
	}
	defer tx.Rollback(r.Context())
	codesJSON, _ := json.Marshal(codes)
	verifiedJSON, _ := json.Marshal(in.MerchantConfirmed)
	modeJSON, _ := json.Marshal(strings.ToLower(strings.TrimSpace(os.Getenv("RAZORPAY_MODE"))))
	for key, value := range map[string][]byte{"razorpay_currency_confirmation_mode": modeJSON, "razorpay_supported_currencies": codesJSON, "razorpay_merchant_currencies_verified": verifiedJSON} {
		if _, err = tx.Exec(r.Context(), `INSERT INTO platform_settings(setting_key,setting_value,updated_at) VALUES($1,$2,NOW()) ON CONFLICT(setting_key) DO UPDATE SET setting_value=EXCLUDED.setting_value,updated_at=NOW()`, key, string(value)); err != nil {
			jsonResponse(w, 500, map[string]string{"error": "Could not save payment currency settings"})
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not save payment currency settings"})
		return
	}
	h.AdminCurrencySettings(w, &http.Request{Method: http.MethodGet, URL: r.URL, Header: r.Header})
}

func mockPaymentsEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "local") &&
		strings.EqualFold(strings.TrimSpace(os.Getenv("PAYMENT_PROVIDER")), "mock")
}

func checkoutEligible(pricingType, period string, price float64) bool {
	return pricingType == "fixed" && price > 0 && (period == "monthly" || period == "yearly" || period == "lifetime")
}

// rejectDuplicatePlanCheckout prevents active-plan repurchases and avoids
// creating multiple open orders for the same user and plan.
func (h *Handler) rejectDuplicatePlanCheckout(w http.ResponseWriter, r *http.Request, userID, planID int64, provider string) bool {
	var active, pending bool
	err := h.DB.QueryRow(r.Context(), `SELECT
		EXISTS(SELECT 1 FROM subscriptions WHERE user_id=$1 AND plan_id=$2 AND status='active' AND (ends_at IS NULL OR ends_at>NOW())),
		EXISTS(SELECT 1 FROM payments WHERE user_id=$1 AND plan_id=$2 AND provider=$3 AND status IN ('created','pending','authorized') AND created_at>NOW()-INTERVAL '30 minutes')`, userID, planID, provider).Scan(&active, &pending)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Could not validate existing subscription"})
		return true
	}
	if active {
		jsonResponse(w, http.StatusConflict, map[string]string{"error": "You already have this plan"})
		return true
	}
	if pending {
		jsonResponse(w, http.StatusConflict, map[string]string{"error": "A checkout for this plan is already pending"})
		return true
	}
	return false
}

func (h *Handler) Provider(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("PAYMENT_PROVIDER")))
	switch provider {
	case "razorpay":
		if _, err := h.client(); err != nil {
			jsonResponse(w, 200, map[string]any{"provider": "unavailable", "mode": strings.ToLower(strings.TrimSpace(os.Getenv("RAZORPAY_MODE"))), "supported_currencies": []string{}, "currency_configuration_source": CurrencyConfigurationSource("razorpay"), "merchant_currencies_verified": false})
			return
		}
		codes, verified, source := h.configuredCurrencies("razorpay")
		jsonResponse(w, 200, map[string]any{"provider": "razorpay", "mode": strings.ToLower(strings.TrimSpace(os.Getenv("RAZORPAY_MODE"))), "supported_currencies": codes, "currency_configuration_source": source, "merchant_currencies_verified": verified})
	case "mock":
		if mockPaymentsEnabled() {
			codes, verified, source := h.configuredCurrencies("mock")
			jsonResponse(w, 200, map[string]any{"provider": "mock", "supported_currencies": codes, "currency_configuration_source": source, "merchant_currencies_verified": verified})
			return
		}
		jsonResponse(w, 200, map[string]any{"provider": "unavailable", "supported_currencies": []string{}, "currency_configuration_source": "provider is not configured", "merchant_currencies_verified": false})
	default:
		jsonResponse(w, 200, map[string]any{"provider": "unavailable", "supported_currencies": []string{}, "currency_configuration_source": "provider is not configured", "merchant_currencies_verified": false})
	}
}

func mockID(prefix string) (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	value := hex.EncodeToString(random)
	uuid := value[:8] + "-" + value[8:12] + "-" + value[12:16] + "-" + value[16:20] + "-" + value[20:]
	return prefix + uuid, nil
}

func (h *Handler) MockOrder(w http.ResponseWriter, r *http.Request) {
	if !mockPaymentsEnabled() {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Demo checkout is unavailable"})
		return
	}
	userID, ok := requestUserID(r)
	if !ok {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "User authentication required"})
		return
	}
	var in struct {
		PlanID int64 `json:"plan_id"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 16<<10))
	dec.DisallowUnknownFields()
	if dec.Decode(&in) != nil || in.PlanID < 1 {
		jsonResponse(w, 400, map[string]string{"error": "A valid plan_id is required"})
		return
	}
	if h.rejectDuplicatePlanCheckout(w, r, userID, in.PlanID, "mock") {
		return
	}
	var name, currency, price, period, pricingType string
	var enabled, userActive bool
	err := h.DB.QueryRow(r.Context(), `SELECT p.name,p.currency,p.price::text,p.billing_period,p.pricing_type,p.is_enabled,(u.status='active') FROM website_pricing_plans p CROSS JOIN users u WHERE p.id=$1 AND u.id=$2`, in.PlanID, userID).Scan(&name, &currency, &price, &period, &pricingType, &enabled, &userActive)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": "Plan or user was not found"})
		return
	}
	if !userActive {
		jsonResponse(w, 403, map[string]string{"error": "User account is not active"})
		return
	}
	if !enabled {
		jsonResponse(w, 400, map[string]string{"error": "This plan is not available"})
		return
	}
	priceValue, priceErr := strconv.ParseFloat(price, 64)
	if priceErr != nil || !checkoutEligible(pricingType, period, priceValue) {
		jsonResponse(w, 400, map[string]string{"error": "This billing period cannot be purchased online"})
		return
	}
	if !h.checkoutCurrencySupported("mock", currency) {
		jsonResponse(w, 400, map[string]string{"error": "This currency is not enabled for the configured payment provider. Please contact support to confirm merchant currency availability or select another plan."})
		return
	}
	orderID, err := mockID("mock_order_")
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not create demo order"})
		return
	}
	var publicID string
	err = h.DB.QueryRow(r.Context(), `INSERT INTO payments(user_id,plan_id,provider,provider_order_id,amount,currency,status) VALUES($1,$2,'mock',$3,$4,$5,'created') RETURNING public_id::text`, userID, in.PlanID, orderID, price, strings.ToUpper(currency)).Scan(&publicID)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not create demo payment record"})
		return
	}
	jsonResponse(w, http.StatusCreated, map[string]any{"mock_order_id": orderID, "payment_public_id": publicID, "plan_id": in.PlanID, "plan_name": name, "amount": price, "currency": strings.ToUpper(currency), "billing_period": period, "demo": true})
}

func (h *Handler) MockVerify(w http.ResponseWriter, r *http.Request) {
	if !mockPaymentsEnabled() {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Demo checkout is unavailable"})
		return
	}
	userID, ok := requestUserID(r)
	if !ok {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "User authentication required"})
		return
	}
	var in struct {
		OrderID string `json:"mock_order_id"`
		Result  string `json:"result"`
		Method  string `json:"payment_method"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 16<<10))
	dec.DisallowUnknownFields()
	if dec.Decode(&in) != nil || dec.Decode(new(any)) != io.EOF || in.OrderID == "" || (in.Result != "success" && in.Result != "failed") || safeMethod(in.Method) == "" {
		jsonResponse(w, 400, map[string]string{"error": "Invalid demo verification request"})
		return
	}
	if !strings.HasPrefix(in.OrderID, "mock_order_") {
		jsonResponse(w, 400, map[string]string{"error": "Invalid demo order"})
		return
	}
	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not verify demo payment"})
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	var id, planID int64
	var orderID, amount, currency, status string
	var previousPaymentID *string
	var orgID *int64
	err = tx.QueryRow(r.Context(), `SELECT id,plan_id,provider_order_id,amount::text,currency,status,provider_payment_id,organization_id FROM payments WHERE provider_order_id=$1 AND user_id=$2 AND provider='mock' FOR UPDATE`, in.OrderID, userID).Scan(&id, &planID, &orderID, &amount, &currency, &status, &previousPaymentID, &orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		jsonResponse(w, 404, map[string]string{"error": "Demo order not found"})
		return
	}
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not load demo order"})
		return
	}
	finalStatus := "failed"
	if in.Result == "success" {
		finalStatus = "paid"
	}
	if status == "paid" || status == "failed" {
		if status != finalStatus {
			jsonResponse(w, http.StatusConflict, map[string]string{"error": "This demo order already has a different final result"})
			return
		}
		if err = tx.Commit(r.Context()); err != nil {
			jsonResponse(w, 500, map[string]string{"error": "Could not complete demo verification"})
			return
		}
		jsonResponse(w, 200, map[string]any{"status": status, "mock_payment_id": previousPaymentID, "payment_method": in.Method, "plan_id": planID, "amount": amount, "currency": currency, "idempotent": true})
		return
	}
	if status != "created" {
		jsonResponse(w, http.StatusConflict, map[string]string{"error": "This demo order cannot be verified"})
		return
	}
	paymentID, err := mockID("mock_pay_")
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not create demo transaction"})
		return
	}
	eventID := "mock-" + in.Result + "-" + orderID
	method := safeMethod(in.Method)
	if in.Result == "failed" {
		payload, _ := json.Marshal(map[string]any{"order_id": orderID, "payment_id": paymentID, "status": "failed", "method": method, "demo": true})
		if _, err = tx.Exec(r.Context(), `UPDATE payments SET status='failed',provider_payment_id=$1,payment_method=$2,updated_at=NOW() WHERE id=$3`, paymentID, method, id); err != nil {
			jsonResponse(w, 500, map[string]string{"error": "Could not record failed demo payment"})
			return
		}
		if _, err = tx.Exec(r.Context(), `INSERT INTO payment_transactions(payment_id,provider_event_id,event_type,amount,currency,payload) VALUES($1,$2,'mock.failed',$3,$4,$5::jsonb)`, id, eventID, amount, currency, string(payload)); err != nil {
			jsonResponse(w, 500, map[string]string{"error": "Could not record demo transaction"})
			return
		}
	} else {
		minor, convErr := razorpay.AmountToMinor(amount, currency)
		if convErr != nil {
			jsonResponse(w, 400, map[string]string{"error": "Plan amount or currency is invalid"})
			return
		}
		p := razorpay.Payment{ID: paymentID, OrderID: orderID, Amount: minor, Currency: strings.ToUpper(currency), Status: "captured", Method: method}
		payload, _ := json.Marshal(map[string]any{"order_id": orderID, "payment_id": paymentID, "method": method, "amount": amount, "currency": currency, "demo": true})
		if err = activateAndRecord(r.Context(), tx, id, userID, orgID, planID, orderID, amount, currency, p, "mock.paid", eventID, payload); err != nil {
			log.Printf("mock payment finalization failed for payment %d: %v", id, err)
			jsonResponse(w, 500, map[string]string{"error": "Could not finalize demo payment"})
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not commit demo payment"})
		return
	}
	jsonResponse(w, 200, map[string]any{"status": finalStatus, "mock_payment_id": paymentID, "payment_method": method, "plan_id": planID, "amount": amount, "currency": currency, "demo": true})
}

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUserID(r)
	if !ok {
		jsonResponse(w, 401, map[string]string{"error": "User authentication required"})
		return
	}
	client, err := h.client()
	if err != nil {
		jsonResponse(w, 503, map[string]string{"error": "Checkout is not configured for this environment"})
		return
	}
	var in struct {
		Currency       string `json:"currency"`
		Country        string `json:"country"`
		ExpectedAmount string `json:"expected_amount"`
		PlanID         int64  `json:"plan_id"`
		BillingPeriod  string `json:"billing_period"`
		PayNow         bool   `json:"pay_now"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 16<<10))
	dec.DisallowUnknownFields()
	if dec.Decode(&in) != nil || dec.Decode(new(any)) != io.EOF || in.PlanID < 1 {
		jsonResponse(w, 400, map[string]string{"error": "A valid plan_id is required"})
		return
	}
	// Load merchant settings before holding a checkout transaction connection.
	// Acquiring another pool connection inside it deadlocks a single-connection
	// pool and can exhaust larger pools under concurrent checkout load.
	allowedCurrencies, _, _ := h.configuredCurrencies("razorpay")
	global, configErr := h.globalConfig(r.Context())
	if configErr != nil {
		jsonResponse(w, 503, map[string]string{"error": "Could not load payment capabilities"})
		return
	}
	country, countryErr := pricing.Country(in.Country)
	if countryErr != nil {
		jsonResponse(w, 400, map[string]string{"error": countryErr.Error()})
		return
	}
	gateway := razorpayGateway{client: client, webhookSecret: os.Getenv("RAZORPAY_WEBHOOK_SECRET")}
	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Unable to start payment. Please try again."})
		return
	}
	defer tx.Rollback(r.Context())
	// Serialize checkout creation with activation for this user, across API instances.
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1::text,0))`, strconv.FormatInt(userID, 10)); err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Unable to start payment. Please try again."})
		return
	}
	var planName, currency, price, period, pricingType, name, email, contact, trialMode string
	var enabled, userActive bool
	err = tx.QueryRow(r.Context(), `SELECT p.name,p.currency,p.price::text,p.billing_period,p.pricing_type,p.is_enabled,(u.status='active'),COALESCE(u.name,''),COALESCE(u.email,''),COALESCE(u.phone,''),p.trial_mode FROM website_pricing_plans p CROSS JOIN users u WHERE p.id=$1 AND u.id=$2`, in.PlanID, userID).Scan(&planName, &currency, &price, &period, &pricingType, &enabled, &userActive, &name, &email, &contact, &trialMode)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": "Plan or user was not found"})
		return
	}
	if !userActive {
		jsonResponse(w, 403, map[string]string{"error": "User account is not active"})
		return
	}
	if !enabled {
		jsonResponse(w, 400, map[string]string{"error": "This plan is not available"})
		return
	}
	var rawConfig []byte
	if err = tx.QueryRow(r.Context(), `SELECT billing_config FROM website_pricing_plans WHERE id=$1`, in.PlanID).Scan(&rawConfig); err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not load plan prices"})
		return
	}
	cfg, cfgErr := pricing.Parse(rawConfig)
	if cfgErr != nil {
		jsonResponse(w, 500, map[string]string{"error": "Invalid plan price configuration"})
		return
	}
	quote, quoteErr := pricing.Resolve(cfg, currency, price, period, in.Currency, country, strings.ToLower(strings.TrimSpace(in.BillingPeriod)))
	if quoteErr != nil {
		status := 400
		if in.BillingPeriod != "" && in.BillingPeriod != period {
			status = 409
		}
		jsonResponse(w, status, map[string]string{"error": quoteErr.Error()})
		return
	}
	if !contains(quote.Providers, "razorpay") {
		jsonResponse(w, 400, map[string]string{"error": "This provider is not enabled for the selected plan"})
		return
	}
	currency, price, period = quote.Currency, quote.Total, quote.Period
	if in.ExpectedAmount != "" {
		expected, expectedErr := pricing.Minor(in.ExpectedAmount, currency)
		if expectedErr != nil || expected != quote.Minor {
			jsonResponse(w, 409, map[string]any{"error": "The plan total changed. Review the updated amount and confirm again.", "quote": quote})
			return
		}
	}
	methods, capErr := global.Check("razorpay", currency, country, quote.Minor)
	if capErr != nil {
		jsonResponse(w, 400, map[string]string{"error": capErr.Error()})
		return
	}
	if (trialMode == "limited" || trialMode == "unlimited") && !in.PayNow {
		jsonResponse(w, http.StatusConflict, map[string]string{"error": "Please choose a free trial or agree to pay now before continuing to checkout."})
		return
	}
	priceValue, priceErr := strconv.ParseFloat(price, 64)
	if priceErr != nil || !checkoutEligible(pricingType, period, priceValue) {
		jsonResponse(w, 400, map[string]string{"error": "This billing period cannot be purchased online"})
		return
	}
	var active bool
	if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM subscriptions WHERE user_id=$1 AND plan_id=$2 AND status='active' AND currency=$3 AND billing_period=$4 AND (ends_at IS NULL OR ends_at>NOW()))`, userID, in.PlanID, currency, period).Scan(&active); err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not validate existing subscription"})
		return
	}
	if active {
		jsonResponse(w, 409, map[string]string{"error": "You already have this plan"})
		return
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	var publicID, orderID string
	var stalePending bool
	var authorizedMismatch bool
	if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM payments WHERE user_id=$1 AND provider='razorpay' AND status='authorized' AND subscription_id IS NULL AND (plan_id<>$2 OR UPPER(currency)<>$3 OR amount<>$4::numeric OR COALESCE(billing_period,$5)<>$5))`, userID, in.PlanID, currency, price, period).Scan(&authorizedMismatch); err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not validate pending checkout"})
		return
	}
	if authorizedMismatch {
		jsonResponse(w, http.StatusConflict, map[string]string{"error": "A previous payment is already authorized. Refresh billing or contact support before starting another checkout."})
		return
	}
	if staleErr := guardStaleOrders(r.Context(), tx, gateway, userID, in.PlanID, currency, price, period, country); staleErr != nil {
		jsonResponse(w, 409, map[string]string{"error": staleErr.Error()})
		return
	}
	invalidated, err := tx.Exec(r.Context(), `WITH invalidated AS (UPDATE payments SET status='cancelled',updated_at=NOW() WHERE user_id=$1 AND provider='razorpay' AND status IN ('created','pending') AND subscription_id IS NULL AND (plan_id<>$2 OR UPPER(currency)<>$3 OR amount<>$4::numeric OR COALESCE(billing_period,$5)<>$5 OR checkout_country<>$6 OR created_at<NOW()-INTERVAL '30 minutes') RETURNING id,amount,currency) INSERT INTO payment_transactions(payment_id,event_type,amount,currency,payload) SELECT id,'checkout.invalidated',amount,currency,'{"reason":"quote_changed_or_expired"}'::jsonb FROM invalidated`, userID, in.PlanID, currency, price, period, country)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not invalidate the previous checkout"})
		return
	}
	stalePending = invalidated.RowsAffected() > 0
	// Reuse only an order whose amount, currency and billing period still match
	// the authoritative plan row. Paid and cancelled payments are never reused.
	responseStatus := 201
	err = tx.QueryRow(r.Context(), `SELECT public_id::text,provider_order_id FROM payments WHERE user_id=$1 AND plan_id=$2 AND provider='razorpay' AND status IN ('created','pending') AND subscription_id IS NULL AND UPPER(currency)=$3 AND amount=$4::numeric AND COALESCE(billing_period,$5)=$5 AND checkout_country=$6 AND created_at>NOW()-INTERVAL '30 minutes' ORDER BY created_at DESC LIMIT 1`, userID, in.PlanID, currency, price, period, country).Scan(&publicID, &orderID)
	if err == nil {
		responseStatus = 200
	} else if !errors.Is(err, pgx.ErrNoRows) {
		jsonResponse(w, 500, map[string]string{"error": "Could not load pending checkout"})
		return
	}
	if !currencyInList(allowedCurrencies, currency) {
		if stalePending {
			if err = tx.Commit(r.Context()); err != nil {
				jsonResponse(w, 500, map[string]string{"error": "Could not invalidate the previous checkout"})
				return
			}
		}
		jsonResponse(w, 400, map[string]string{"error": "This currency is not enabled for the configured payment provider. Please contact support to confirm merchant currency availability or select another plan."})
		return
	}
	minor, convErr := razorpay.CheckoutAmountToMinor(price, currency)
	if convErr != nil || minor <= 0 {
		message := "The payment gateway cannot charge this exact plan amount. Please contact support to review the plan price and currency precision."
		if currency == "INR" && priceValue < 1 {
			message = "Razorpay requires an INR payment of at least ₹1.00. This plan is priced below that minimum; update the plan price before checkout."
		}
		if stalePending {
			if err = tx.Commit(r.Context()); err != nil {
				jsonResponse(w, 500, map[string]string{"error": "Could not invalidate the previous checkout"})
				return
			}
		}
		jsonResponse(w, 400, map[string]string{"error": message})
		return
	}
	if orderID != "" {
		existing, fetchErr := gateway.FetchOrder(r.Context(), orderID)
		if fetchErr != nil {
			jsonResponse(w, 502, map[string]string{"error": "Could not confirm the existing provider order. Please retry."})
			return
		}
		if existing.Status == "paid" || existing.Status == "attempted" {
			jsonResponse(w, 409, map[string]string{"error": "Previous checkout needs payment reconciliation. Refresh billing before retrying."})
			return
		}
		if existing.ID != orderID || existing.Amount != minor || existing.Currency != currency || existing.Status != "created" {
			if _, err = tx.Exec(r.Context(), `WITH invalidated AS (UPDATE payments SET status='cancelled',updated_at=NOW() WHERE provider_order_id=$1 AND status IN ('created','pending') RETURNING id,amount,currency) INSERT INTO payment_transactions(payment_id,event_type,amount,currency,payload) SELECT id,'checkout.provider_mismatch',amount,currency,'{}'::jsonb FROM invalidated`, orderID); err != nil {
				jsonResponse(w, 500, map[string]string{"error": "Could not invalidate provider order"})
				return
			}
			orderID = ""
			stalePending = true
		}
	}
	if orderID == "" {
		receipt := fmt.Sprintf("dc-%d", time.Now().UnixNano())
		order, providerErr := gateway.CreateOrder(r.Context(), minor, currency, receipt)
		if providerErr != nil {
			log.Printf("checkout stage=provider_order user_id=%d plan_id=%d currency=%s billing_period=%s reference=%s: %v", userID, in.PlanID, currency, period, receipt, providerErr)
			code, message := checkoutFailure(providerErr)
			if stalePending {
				if err = tx.Commit(r.Context()); err != nil {
					jsonResponse(w, 500, map[string]string{"error": "Could not invalidate the previous checkout"})
					return
				}
			}
			jsonResponse(w, code, map[string]string{"error": message})
			return
		}
		if order.ID == "" || order.Amount != minor || order.Currency != currency {
			log.Printf("checkout stage=provider_response user_id=%d plan_id=%d currency=%s billing_period=%s reference=%s reason=order_mismatch", userID, in.PlanID, currency, period, receipt)
			jsonResponse(w, 502, map[string]string{"error": "The payment provider returned an invalid checkout. Please retry or contact support."})
			return
		}
		orderID = order.ID
		err = tx.QueryRow(r.Context(), `INSERT INTO payments(user_id,plan_id,provider,provider_order_id,amount,currency,billing_period,checkout_country,checkout_snapshot,status) VALUES($1,$2,'razorpay',$3,$4,$5,$6,$7,$8::jsonb,'created') RETURNING public_id::text`, userID, in.PlanID, orderID, price, currency, period, country, quoteJSON(quote)).Scan(&publicID)
		if err != nil {
			jsonResponse(w, 500, map[string]string{"error": "Could not create local payment record"})
			return
		}
		responseStatus = 201
	}
	if err = tx.Commit(r.Context()); err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not save checkout"})
		return
	}
	jsonResponse(w, responseStatus, map[string]any{"quote": quote, "payment_methods": methods, "renewal": "manual", "payment_id": publicID, "payment_public_id": publicID, "key_id": client.KeyID, "razorpay_order_id": orderID, "amount": minor, "amount_major": price, "currency": currency, "billing_period": period, "plan_name": planName, "mode": strings.ToLower(strings.TrimSpace(os.Getenv("RAZORPAY_MODE"))), "prefill": map[string]string{"name": name, "email": email, "contact": contact}})
}

// ActivateFreePlan creates an active zero-value subscription without creating
// a payment or payment transaction. It shares the per-user advisory lock used
// by checkout so simultaneous activation/purchase requests cannot race.
func (h *Handler) ActivateFreePlan(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUserID(r)
	if !ok {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "User authentication required"})
		return
	}
	var in struct {
		PlanID int64 `json:"plan_id"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 16<<10))
	dec.DisallowUnknownFields()
	if dec.Decode(&in) != nil || dec.Decode(new(any)) != io.EOF || in.PlanID < 1 {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "A valid plan_id is required"})
		return
	}
	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Could not activate free plan"})
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1::text,0))`, strconv.FormatInt(userID, 10)); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Could not activate free plan"})
		return
	}
	var name, pricingType, period, currency, trialMode string
	var trialDays int
	var price float64
	var enabled, userActive bool
	err = tx.QueryRow(r.Context(), `SELECT p.name,p.pricing_type,p.billing_period,p.price,p.currency,p.is_enabled,(u.status='active'),p.trial_mode,p.trial_days FROM website_pricing_plans p CROSS JOIN users u WHERE p.id=$1 AND u.id=$2`, in.PlanID, userID).Scan(&name, &pricingType, &period, &price, &currency, &enabled, &userActive, &trialMode, &trialDays)
	if err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Plan or user was not found"})
		return
	}
	if !userActive {
		jsonResponse(w, http.StatusForbidden, map[string]string{"error": "User account is not active"})
		return
	}
	if !enabled {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "This plan is not available"})
		return
	}
	trial := price > 0 && (trialMode == "limited" || trialMode == "unlimited")
	unlimitedTrial := trialMode == "unlimited"
	if (!trial && (pricingType != "free" || price != 0)) || period == "custom" || (trialMode == "limited" && trialDays < 1) {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Only enabled free plans can be activated without checkout"})
		return
	}
	var active bool
	if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM subscriptions WHERE user_id=$1 AND plan_id=$2 AND status='active' AND (ends_at IS NULL OR ends_at>NOW()))`, userID, in.PlanID).Scan(&active); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Could not validate existing subscription"})
		return
	}
	if active {
		jsonResponse(w, http.StatusConflict, map[string]string{"error": "You already have this plan"})
		return
	}
	if trial {
		var used bool
		if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM subscriptions s LEFT JOIN website_pricing_plans p ON p.id=s.plan_id WHERE s.user_id=$1 AND (s.is_trial OR (p.price>0 AND p.pricing_type<>'free' AND (s.status='trial' OR s.trial_ends_at IS NOT NULL))))`, userID).Scan(&used); err != nil {
			jsonResponse(w, 500, map[string]string{"error": "Could not validate trial eligibility"})
			return
		}
		if used {
			jsonResponse(w, http.StatusConflict, map[string]string{"error": "You have already used a free trial. Please choose paid checkout."})
			return
		}
	}
	if _, err = tx.Exec(r.Context(), `UPDATE subscriptions SET status='cancelled',updated_at=NOW() WHERE user_id=$1 AND status IN ('active','trial')`, userID); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Could not activate free plan"})
		return
	}
	var subscriptionID int64
	status := "active"
	amount := price
	var trialEnd *time.Time
	if trial {
		status = "trial"
		amount = 0
		if !unlimitedTrial {
			end := time.Now().UTC().Add(time.Duration(trialDays) * 24 * time.Hour)
			trialEnd = &end
		}
	}
	err = tx.QueryRow(r.Context(), `INSERT INTO subscriptions(user_id,plan_id,status,started_at,trial_ends_at,amount,currency,billing_period,is_trial) VALUES($1,$2,$3,NOW(),$4,$5,$6,$7,$8) RETURNING id`, userID, in.PlanID, status, trialEnd, amount, strings.ToUpper(currency), period, trial).Scan(&subscriptionID)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Could not activate free plan"})
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Could not activate free plan"})
		return
	}
	jsonResponse(w, http.StatusCreated, map[string]any{"status": status, "subscription_id": subscriptionID, "plan_id": in.PlanID, "plan_name": name, "amount": amount, "currency": strings.ToUpper(currency), "trial_mode": trialMode, "trial_ends_at": trialEnd})
}

type verifyRequest struct {
	PaymentID string `json:"razorpay_payment_id"`
	OrderID   string `json:"razorpay_order_id"`
	Signature string `json:"razorpay_signature"`
}

func (h *Handler) VerifyCheckout(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUserID(r)
	if !ok {
		jsonResponse(w, 401, map[string]string{"error": "User authentication required"})
		return
	}
	client, err := h.client()
	if err != nil {
		jsonResponse(w, 503, map[string]string{"error": "Checkout is not configured for this environment"})
		return
	}
	var in verifyRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, 16<<10))
	dec.DisallowUnknownFields()
	if dec.Decode(&in) != nil || dec.Decode(new(any)) != io.EOF || in.PaymentID == "" || in.OrderID == "" || in.Signature == "" {
		jsonResponse(w, 400, map[string]string{"error": "Invalid payment verification request"})
		return
	}
	var id, planID int64
	var amountText, currency, status, expectedOrder string
	var paidAt *time.Time
	var publicID string
	var subscriptionID *int64
	err = h.DB.QueryRow(r.Context(), `SELECT id,public_id::text,plan_id,provider_order_id,amount::text,currency,status,paid_at,subscription_id FROM payments WHERE provider_order_id=$1 AND user_id=$2 AND provider='razorpay'`, in.OrderID, userID).Scan(&id, &publicID, &planID, &expectedOrder, &amountText, &currency, &status, &paidAt, &subscriptionID)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": "Payment not found"})
		return
	}
	secret := strings.TrimSpace(os.Getenv("RAZORPAY_KEY_SECRET"))
	if in.OrderID != expectedOrder || !razorpay.VerifyCheckoutSignature(expectedOrder, in.PaymentID, in.Signature, secret) {
		jsonResponse(w, 400, map[string]string{"error": "Payment verification failed"})
		return
	}
	if status == "paid" && subscriptionID == nil {
		jsonResponse(w, 409, map[string]string{"error": "Payment was captured but needs reconciliation. Please contact support before retrying."})
		return
	}
	if status == "paid" || status == "refunded" {
		jsonResponse(w, 200, map[string]any{"status": status, "payment_public_id": publicID, "paid_at": paidAt})
		return
	}
	if status == "cancelled" {
		jsonResponse(w, http.StatusConflict, map[string]string{"error": "This payment was cancelled. Please refresh billing or contact support before retrying."})
		return
	}
	providerPayment, err := client.FetchPayment(r.Context(), in.PaymentID)
	if err != nil {
		jsonResponse(w, 502, map[string]string{"error": "Could not verify payment with Razorpay"})
		return
	}
	minor, err := razorpay.AmountToMinor(amountText, currency)
	if err != nil || providerPayment.ID != in.PaymentID || providerPayment.OrderID != expectedOrder || providerPayment.Amount != minor || !strings.EqualFold(providerPayment.Currency, currency) {
		jsonResponse(w, 400, map[string]string{"error": "Payment does not match the expected order"})
		return
	}
	if providerPayment.Status != "captured" {
		jsonResponse(w, 202, map[string]any{"status": providerPayment.Status, "payment_public_id": publicID, "message": "Payment is not captured yet"})
		return
	}
	result, err := h.recordPaid(r.Context(), id, providerPayment, "checkout.verified", nil)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not finalize verified payment"})
		return
	}
	if result["status"] == "cancelled" || (result["status"] == "paid" && result["id"] == (*int64)(nil)) {
		jsonResponse(w, http.StatusConflict, map[string]string{"error": "This payment was cancelled. Please refresh billing or contact support before retrying."})
		return
	}
	jsonResponse(w, 200, map[string]any{"status": result["status"], "payment_public_id": publicID, "payment_method": safeMethod(providerPayment.Method), "subscription": result})
}

func safeMethod(v string) string {
	switch strings.ToLower(v) {
	case "upi", "netbanking", "card", "wallet", "emi", "paylater", "bank_transfer":
		return strings.ToLower(v)
	default:
		return ""
	}
}

func (h *Handler) recordPaid(ctx context.Context, paymentID int64, p razorpay.Payment, eventType string, eventPayload map[string]any) (map[string]any, error) {
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = h.finalizeCapturedTx(ctx, tx, paymentID, p, eventType, "checkout-"+p.ID); err != nil {
		return nil, err
	}
	var subID *int64
	var status string
	if err = tx.QueryRow(ctx, `SELECT subscription_id,status FROM payments WHERE id=$1`, paymentID).Scan(&subID, &status); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"id": subID, "status": status}, nil
}

type webhookEnvelope struct {
	Event   string `json:"event"`
	Payload struct {
		Payment struct {
			Entity razorpay.Payment `json:"entity"`
		} `json:"payment"`
		Order struct {
			Entity struct {
				ID string `json:"id"`
			} `json:"entity"`
		} `json:"order"`
		Refund struct {
			Entity struct {
				ID        string `json:"id"`
				PaymentID string `json:"payment_id"`
				Amount    int64  `json:"amount"`
				Currency  string `json:"currency"`
				Status    string `json:"status"`
			} `json:"entity"`
		} `json:"refund"`
	} `json:"payload"`
}

func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil || len(body) == 0 {
		jsonResponse(w, 400, map[string]string{"error": "Invalid webhook body"})
		return
	}
	if !razorpay.VerifyWebhookSignature(body, r.Header.Get("X-Razorpay-Signature"), strings.TrimSpace(os.Getenv("RAZORPAY_WEBHOOK_SECRET"))) {
		jsonResponse(w, 400, map[string]string{"error": "Invalid webhook signature"})
		return
	}
	var event webhookEnvelope
	if json.Unmarshal(body, &event) != nil {
		jsonResponse(w, 400, map[string]string{"error": "Invalid webhook event"})
		return
	}
	eventID := strings.TrimSpace(r.Header.Get("X-Razorpay-Event-Id"))
	if eventID == "" {
		sum := sha256.Sum256(body)
		eventID = hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(body)
	fingerprint := hex.EncodeToString(sum[:])
	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not process webhook"})
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	tag, err := tx.Exec(r.Context(), `INSERT INTO processed_payment_webhook_events(provider_event_id,event_type,payload_sha256) VALUES($1,$2,$3) ON CONFLICT(provider_event_id) DO NOTHING`, eventID, event.Event, fingerprint)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not record webhook"})
		return
	}
	if tag.RowsAffected() == 0 {
		jsonResponse(w, 200, map[string]string{"status": "duplicate_ignored"})
		return
	}
	// Claim the event and perform its effects under one transaction. Handlers below use the
	// same transaction so a crash cannot leave an event marked processed but unapplied.
	if err = h.processWebhookTx(r, tx, event, eventID, body); err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not apply verified webhook"})
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not commit webhook"})
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "processed"})
}

func (h *Handler) processWebhookTx(r *http.Request, tx pgx.Tx, event webhookEnvelope, eventID string, raw []byte) error {
	if event.Event == "refund.processed" {
		refund := event.Payload.Refund.Entity
		if refund.ID == "" || refund.PaymentID == "" || refund.Status != "processed" || refund.Amount <= 0 {
			return errors.New("invalid refund event")
		}
		var paymentID int64
		var currency, amount string
		err := tx.QueryRow(r.Context(), `SELECT id,currency,amount::text FROM payments WHERE provider_payment_id=$1 AND provider='razorpay' FOR UPDATE`, refund.PaymentID).Scan(&paymentID, &currency, &amount)
		if err != nil {
			return err
		}
		if refund.Currency != "" && !strings.EqualFold(refund.Currency, currency) {
			return errors.New("refund currency mismatch")
		}
		minor, err := razorpay.AmountToMinor(amount, currency)
		if err != nil || refund.Amount > minor {
			return errors.New("refund amount mismatch")
		}
		var duplicate bool
		if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM payment_transactions WHERE payment_id=$1 AND event_type='refund.processed' AND payload->>'refund_id'=$2)`, paymentID, refund.ID).Scan(&duplicate); err != nil {
			return err
		}
		if duplicate {
			return nil
		}
		factor := int64(1)
		for i := 0; i < razorpay.CurrencyExponent(currency); i++ {
			factor *= 10
		}
		safe, _ := json.Marshal(map[string]any{"refund_id": refund.ID, "status": refund.Status, "amount": refund.Amount, "currency": currency})
		if _, err = tx.Exec(r.Context(), `INSERT INTO payment_transactions(payment_id,provider_event_id,event_type,amount,currency,payload) VALUES($1,$2,'refund.processed',$3::numeric/$4::numeric,$5,$6::jsonb)`, paymentID, eventID, refund.Amount, factor, currency, string(safe)); err != nil {
			return err
		}
		// A partial refund is recorded without claiming the full payment was refunded.
		_, err = tx.Exec(r.Context(), `UPDATE payments SET status='refunded',updated_at=NOW() WHERE id=$1 AND (SELECT COALESCE(SUM(amount),0) FROM payment_transactions WHERE payment_id=$1 AND event_type='refund.processed')>=amount`, paymentID)
		return err
	}
	switch event.Event {
	case "payment.captured", "order.paid", "payment.authorized", "payment.failed":
	default:
		return nil
	}
	p := event.Payload.Payment.Entity
	if p.OrderID == "" {
		p.OrderID = event.Payload.Order.Entity.ID
	}
	if p.OrderID == "" {
		return nil
	}
	var paymentID int64
	var amountText, currency, status string
	err := tx.QueryRow(r.Context(), `SELECT id,amount::text,currency,status FROM payments WHERE provider_order_id=$1 AND provider='razorpay' FOR UPDATE`, p.OrderID).Scan(&paymentID, &amountText, &currency, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("local payment not yet available")
	}
	if err != nil {
		return err
	}
	if event.Event == "payment.captured" || event.Event == "order.paid" {
		if p.Status != "captured" || p.ID == "" {
			return errors.New("invalid capture event")
		}
		// The surrounding webhook transaction owns the lock; record the successful event and
		// finalize subscription in a nested helper that uses this transaction.
		return h.finalizeCapturedTx(r.Context(), tx, paymentID, p, event.Event, eventID)
	}
	expected, amountErr := razorpay.AmountToMinor(amountText, currency)
	if amountErr != nil || p.Amount != expected || !strings.EqualFold(p.Currency, currency) {
		return errors.New("provider event amount or currency mismatch")
	}
	newStatus := ""
	switch event.Event {
	case "payment.authorized":
		newStatus = "authorized"
	case "payment.failed":
		newStatus = "failed"
	default:
		return nil
	}
	if status != "paid" && status != "refunded" && status != "cancelled" {
		_, err = tx.Exec(r.Context(), `UPDATE payments SET status=$1,provider_payment_id=COALESCE(NULLIF($2,''),provider_payment_id),payment_method=NULLIF($3,''),bank=NULLIF($4,''),wallet=NULLIF($5,''),error_code=NULLIF($6,''),error_description=NULLIF($7,''),updated_at=NOW() WHERE id=$8`, newStatus, p.ID, safeMethod(p.Method), p.Bank, p.Wallet, p.ErrorCode, p.ErrorDescription, paymentID)
		if err != nil {
			return err
		}
	}
	amount, _ := strconv.ParseFloat(amountText, 64)
	clean, _ := json.Marshal(map[string]any{"order_id": p.OrderID, "payment_id": p.ID, "status": newStatus, "method": safeMethod(p.Method), "error_code": p.ErrorCode, "error_description": p.ErrorDescription})
	_, err = tx.Exec(r.Context(), `INSERT INTO payment_transactions(payment_id,provider_event_id,event_type,amount,currency,payload) VALUES($1,$2,$3,$4,$5,$6::jsonb) ON CONFLICT(provider_event_id) WHERE provider_event_id IS NOT NULL DO NOTHING`, paymentID, eventID, event.Event, amount, currency, string(clean))
	return err
}

func (h *Handler) finalizeCapturedTx(ctx context.Context, tx pgx.Tx, paymentID int64, p razorpay.Payment, eventType, eventID string) error {
	var userID, planID int64
	var orgID *int64
	var order, currency, amountText, status string
	err := tx.QueryRow(ctx, `SELECT user_id,organization_id,plan_id,provider_order_id,currency,amount::text,status FROM payments WHERE id=$1 FOR UPDATE`, paymentID).Scan(&userID, &orgID, &planID, &order, &currency, &amountText, &status)
	if err != nil {
		return err
	}
	minor, err := razorpay.AmountToMinor(amountText, currency)
	if err != nil || p.ID == "" || p.Status != "captured" || p.OrderID != order || p.Amount != minor || !strings.EqualFold(p.Currency, currency) {
		return errors.New("webhook payment mismatch")
	}
	clean, _ := json.Marshal(map[string]any{"order_id": p.OrderID, "payment_id": p.ID, "method": safeMethod(p.Method), "bank": p.Bank, "wallet": p.Wallet, "amount": p.Amount, "currency": p.Currency})
	if status == "cancelled" {
		if err = recordCancelledCapture(ctx, tx, paymentID, p.ID, p.Method); err != nil {
			return err
		}
	}
	if status == "paid" || status == "refunded" || status == "cancelled" {
		_, err = tx.Exec(ctx, `INSERT INTO payment_transactions(payment_id,provider_event_id,event_type,amount,currency,payload) VALUES($1,$2,$3,$4,$5,$6::jsonb) ON CONFLICT(provider_event_id) WHERE provider_event_id IS NOT NULL DO NOTHING`, paymentID, eventID, eventType, amountText, currency, string(clean))
		return err
	}
	// Reuse common subscription activation logic without allowing this path to commit its own transaction.
	return activateAndRecord(ctx, tx, paymentID, userID, orgID, planID, order, amountText, currency, p, eventType, eventID, clean)
}

func (h *Handler) AdminPaymentSummary(w http.ResponseWriter, r *http.Request) {
	var total, paid, failed, pending int64
	var revenue map[string]float64 = map[string]float64{}
	err := h.DB.QueryRow(r.Context(), `SELECT COUNT(*),COUNT(*) FILTER(WHERE status='paid'),COUNT(*) FILTER(WHERE status='failed'),COUNT(*) FILTER(WHERE status IN ('created','pending','authorized')) FROM payments`).Scan(&total, &paid, &failed, &pending)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not load payment summary"})
		return
	}
	rows, err := h.DB.Query(r.Context(), `SELECT currency,SUM(amount) FROM payments WHERE status='paid' GROUP BY currency ORDER BY currency`)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not load verified revenue"})
		return
	}
	defer rows.Close()
	for rows.Next() {
		var currency string
		var amount float64
		if rows.Scan(&currency, &amount) != nil {
			jsonResponse(w, 500, map[string]string{"error": "Could not read verified revenue"})
			return
		}
		revenue[currency] = amount
	}
	jsonResponse(w, 200, map[string]any{"total_payment_attempts": total, "successful_payments": paid, "failed_payments": failed, "pending_payments": pending, "verified_revenue": revenue})
}

func (h *Handler) AdminPayments(w http.ResponseWriter, r *http.Request) {
	search := "%" + strings.TrimSpace(r.URL.Query().Get("search")) + "%"
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	provider := strings.TrimSpace(r.URL.Query().Get("provider"))
	currency := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("currency")))
	method := strings.TrimSpace(r.URL.Query().Get("payment_method"))
	plan := strings.TrimSpace(r.URL.Query().Get("plan_id"))
	from := strings.TrimSpace(r.URL.Query().Get("from"))
	to := strings.TrimSpace(r.URL.Query().Get("to"))
	rows, err := h.DB.Query(r.Context(), `SELECT p.id,p.public_id::text,p.user_id,p.organization_id,p.subscription_id,p.plan_id,COALESCE(u.name,''),COALESCE(u.email,''),COALESCE(o.name,''),COALESCE(pl.name,''),p.amount,p.currency,p.provider,p.provider_order_id,p.payment_method,p.status,p.provider_payment_id,p.paid_at,p.created_at FROM payments p JOIN users u ON u.id=p.user_id LEFT JOIN organizations o ON o.id=p.organization_id JOIN website_pricing_plans pl ON pl.id=p.plan_id WHERE ($1='%%' OR p.public_id::text ILIKE $1 OR COALESCE(u.name,'') ILIKE $1 OR COALESCE(u.email,'') ILIKE $1 OR COALESCE(p.provider_order_id,'') ILIKE $1 OR COALESCE(p.provider_payment_id,'') ILIKE $1) AND ($2='' OR p.status=$2) AND ($3='' OR p.provider=$3) AND ($4='' OR UPPER(p.currency)=$4) AND ($5='' OR p.payment_method=$5) AND ($6='' OR p.plan_id::text=$6) AND ($7='' OR p.created_at >= $7::date) AND ($8='' OR p.created_at < ($8::date + INTERVAL '1 day')) ORDER BY p.created_at DESC LIMIT 500`, search, status, provider, currency, method, plan, from, to)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not load payments"})
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, userID, planID int64
		var orgID, subID *int64
		var publicID, name, email, orgName, planName, currency, paymentProvider, status string
		var amount float64
		var providerOrder *string
		var method, payID *string
		var paidAt *time.Time
		var created time.Time
		if rows.Scan(&id, &publicID, &userID, &orgID, &subID, &planID, &name, &email, &orgName, &planName, &amount, &currency, &paymentProvider, &providerOrder, &method, &status, &payID, &paidAt, &created) != nil {
			jsonResponse(w, 500, map[string]string{"error": "Could not read payments"})
			return
		}
		items = append(items, map[string]any{"id": id, "public_id": publicID, "user_id": userID, "organization_id": orgID, "subscription_id": subID, "plan_id": planID, "subscriber_name": name, "subscriber_email": email, "organization_name": orgName, "plan_name": planName, "amount": amount, "currency": currency, "provider": paymentProvider, "provider_order_id": providerOrder, "payment_method": method, "status": status, "provider_payment_id": payID, "paid_at": paidAt, "created_at": created})
	}
	if rows.Err() != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not read payments"})
		return
	}
	jsonResponse(w, 200, map[string]any{"data": items})
}

func (h *Handler) AdminPaymentDetails(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		jsonResponse(w, 400, map[string]string{"error": "Invalid payment id"})
		return
	}
	out := map[string]any{}
	var publicID, subscriberName, subscriberEmail, organizationName, planName, currency, provider, status string
	var userID, planID int64
	var organizationID, subscriptionID *int64
	var paidAt *time.Time
	var created, updated time.Time
	var amount float64
	var paymentMethod, bank, wallet, errorCode, errorDesc, providerOrder, providerPayment *string
	var subscriptionStatus string
	var subscriptionStartedAt, subscriptionEndsAt *time.Time
	err = h.DB.QueryRow(r.Context(), `SELECT p.id,p.public_id::text,p.user_id,p.organization_id,p.subscription_id,p.plan_id,COALESCE(u.name,''),COALESCE(u.email,''),COALESCE(o.name,''),COALESCE(pl.name,''),p.amount,p.currency,p.provider,p.provider_order_id,p.provider_payment_id,p.payment_method,p.bank,p.wallet,p.status,p.error_code,p.error_description,p.paid_at,p.created_at,p.updated_at,COALESCE(s.status,''),s.started_at,s.ends_at FROM payments p JOIN users u ON u.id=p.user_id LEFT JOIN organizations o ON o.id=p.organization_id JOIN website_pricing_plans pl ON pl.id=p.plan_id LEFT JOIN subscriptions s ON s.id=p.subscription_id WHERE p.id=$1`, id).Scan(&id, &publicID, &userID, &organizationID, &subscriptionID, &planID, &subscriberName, &subscriberEmail, &organizationName, &planName, &amount, &currency, &provider, &providerOrder, &providerPayment, &paymentMethod, &bank, &wallet, &status, &errorCode, &errorDesc, &paidAt, &created, &updated, &subscriptionStatus, &subscriptionStartedAt, &subscriptionEndsAt)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": "Payment not found"})
		return
	}
	out["id"] = id
	out["public_id"] = publicID
	out["user_id"] = userID
	out["organization_id"] = organizationID
	out["subscription_id"] = subscriptionID
	out["subscription"] = map[string]any{"status": subscriptionStatus, "started_at": subscriptionStartedAt, "ends_at": subscriptionEndsAt}
	out["plan_id"] = planID
	out["subscriber_name"] = subscriberName
	out["subscriber_email"] = subscriberEmail
	out["organization_name"] = organizationName
	out["plan_name"] = planName
	out["currency"] = currency
	out["provider"] = provider
	out["status"] = status
	out["amount"] = amount
	out["provider_order_id"] = providerOrder
	out["provider_payment_id"] = providerPayment
	out["payment_method"] = paymentMethod
	out["bank"] = bank
	out["wallet"] = wallet
	out["error_code"] = errorCode
	out["error_description"] = errorDesc
	out["paid_at"] = paidAt
	out["created_at"] = created
	out["updated_at"] = updated
	out["transactions"] = h.paymentTransactions(r, id)
	jsonResponse(w, 200, map[string]any{"data": out})
}

// CancelAdminPayment locally closes an open checkout without claiming that a
// provider-side order was voided. A later capture webhook still wins.
func (h *Handler) CancelAdminPayment(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid payment id"})
		return
	}
	var in struct {
		Status string `json:"status"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 8<<10))
	dec.DisallowUnknownFields()
	if dec.Decode(&in) != nil || dec.Decode(new(any)) != io.EOF || in.Status != "cancelled" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Only cancellation is supported"})
		return
	}
	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Could not cancel payment"})
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	var status, amount, currency string
	var subscriptionID *int64
	err = tx.QueryRow(r.Context(), `SELECT status,amount::text,currency,subscription_id FROM payments WHERE id=$1 FOR UPDATE`, id).Scan(&status, &amount, &currency, &subscriptionID)
	if errors.Is(err, pgx.ErrNoRows) {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Payment not found"})
		return
	}
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Could not load payment"})
		return
	}
	if status == "cancelled" {
		if err = tx.Commit(r.Context()); err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Could not cancel payment"})
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"status": "cancelled", "idempotent": true})
		return
	}
	if (status != "created" && status != "pending") || subscriptionID != nil {
		jsonResponse(w, http.StatusConflict, map[string]string{"error": "Only unlinked created or pending payments can be cancelled"})
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE payments SET status='cancelled',updated_at=NOW() WHERE id=$1`, id); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Could not cancel payment"})
		return
	}
	payload, _ := json.Marshal(map[string]any{"status": "cancelled", "source": "admin"})
	if _, err = tx.Exec(r.Context(), `INSERT INTO payment_transactions(payment_id,event_type,amount,currency,payload) VALUES($1,'admin.cancelled',$2,$3,$4::jsonb)`, id, amount, currency, string(payload)); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Could not record payment cancellation"})
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Could not cancel payment"})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// DeleteAdminPayment removes only non-final, unlinked payment attempts. Paid or
// subscription-linked records remain part of the billing audit history.
func (h *Handler) DeleteAdminPayment(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		jsonResponse(w, 400, map[string]string{"error": "Invalid payment id"})
		return
	}
	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not delete payment record"})
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	var status string
	var subscriptionID *int64
	err = tx.QueryRow(r.Context(), `SELECT status,subscription_id FROM payments WHERE id=$1 FOR UPDATE`, id).Scan(&status, &subscriptionID)
	if errors.Is(err, pgx.ErrNoRows) {
		jsonResponse(w, 404, map[string]string{"error": "Payment not found"})
		return
	}
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not load payment record"})
		return
	}
	if !paymentRecordDeletable(status) {
		message := "Only created, pending, failed, or cancelled payments can be deleted"
		if status == "paid" || status == "captured" || status == "refunded" || status == "partially_refunded" {
			message = "Completed payments cannot be deleted"
		}
		jsonResponse(w, http.StatusConflict, map[string]string{"error": message})
		return
	}
	if subscriptionID != nil {
		jsonResponse(w, http.StatusConflict, map[string]string{"error": "Payments linked to subscriptions cannot be deleted"})
		return
	}
	if _, err = tx.Exec(r.Context(), `DELETE FROM payments WHERE id=$1`, id); err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not delete payment record"})
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not delete payment record"})
		return
	}
	jsonResponse(w, 200, map[string]string{"message": "Payment record deleted"})
}

func paymentRecordDeletable(status string) bool {
	switch status {
	case "created", "pending", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func (h *Handler) paymentTransactions(r *http.Request, paymentID int64) []map[string]any {
	rows, err := h.DB.Query(r.Context(), `SELECT id,provider_event_id,event_type,amount,currency,payload,created_at FROM payment_transactions WHERE payment_id=$1 ORDER BY created_at DESC,id DESC`, paymentID)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var eventID *string
		var event, currency string
		var amount float64
		var payload []byte
		var created time.Time
		if rows.Scan(&id, &eventID, &event, &amount, &currency, &payload, &created) != nil {
			continue
		}
		var safe any = map[string]any{}
		_ = json.Unmarshal(payload, &safe)
		out = append(out, map[string]any{"id": id, "provider_event_id": eventID, "event_type": event, "amount": amount, "currency": currency, "payload": safe, "created_at": created})
	}
	return out
}

func (h *Handler) AdminTransactions(w http.ResponseWriter, r *http.Request) {
	search := "%" + strings.TrimSpace(r.URL.Query().Get("search")) + "%"
	event := strings.TrimSpace(r.URL.Query().Get("event_type"))
	rows, err := h.DB.Query(r.Context(), `SELECT t.id,t.provider_event_id,t.payment_id,p.public_id::text,COALESCE(u.name,''),COALESCE(u.email,''),t.event_type,t.amount,t.currency,p.status,t.created_at FROM payment_transactions t JOIN payments p ON p.id=t.payment_id JOIN users u ON u.id=p.user_id WHERE ($1='%%' OR t.provider_event_id ILIKE $1 OR p.public_id::text ILIKE $1 OR COALESCE(u.name,'') ILIKE $1 OR COALESCE(u.email,'') ILIKE $1) AND ($2='' OR t.event_type=$2) ORDER BY t.created_at DESC LIMIT 1000`, search, event)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not load payment transactions"})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, paymentID int64
		var eventID *string
		var publicID, name, email, event, currency, status string
		var amount float64
		var created time.Time
		if rows.Scan(&id, &eventID, &paymentID, &publicID, &name, &email, &event, &amount, &currency, &status, &created) != nil {
			jsonResponse(w, 500, map[string]string{"error": "Could not read payment transactions"})
			return
		}
		out = append(out, map[string]any{"id": id, "provider_event_id": eventID, "payment_id": paymentID, "payment_public_id": publicID, "subscriber_name": name, "subscriber_email": email, "event_type": event, "amount": amount, "currency": currency, "status": status, "created_at": created})
	}
	jsonResponse(w, 200, map[string]any{"data": out})
}

func (h *Handler) AdminTransactionDetails(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		jsonResponse(w, 400, map[string]string{"error": "Invalid transaction id"})
		return
	}
	var paymentID int64
	var eventID *string
	var paymentPublicID, subscriber, subscriberEmail, event, currency, paymentStatus string
	var amount float64
	var payload []byte
	var created time.Time
	err = h.DB.QueryRow(r.Context(), `SELECT t.id,t.provider_event_id,t.payment_id,p.public_id::text,COALESCE(u.name,''),COALESCE(u.email,''),t.event_type,t.amount,t.currency,p.status,t.payload,t.created_at FROM payment_transactions t JOIN payments p ON p.id=t.payment_id JOIN users u ON u.id=p.user_id WHERE t.id=$1`, id).Scan(&id, &eventID, &paymentID, &paymentPublicID, &subscriber, &subscriberEmail, &event, &amount, &currency, &paymentStatus, &payload, &created)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": "Transaction not found"})
		return
	}
	var safe any = map[string]any{}
	_ = json.Unmarshal(payload, &safe)
	jsonResponse(w, 200, map[string]any{"data": map[string]any{"id": id, "provider_event_id": eventID, "payment_id": paymentID, "payment_public_id": paymentPublicID, "subscriber_name": subscriber, "subscriber_email": subscriberEmail, "event_type": event, "amount": amount, "currency": currency, "payment_status": paymentStatus, "payload": safe, "created_at": created}})
}

func (h *Handler) UserBilling(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUserID(r)
	if !ok {
		jsonResponse(w, 401, map[string]string{"error": "User authentication required"})
		return
	}
	rows, err := h.DB.Query(r.Context(), `SELECT s.id,s.public_id::text,s.plan_id,COALESCE(p.name,''),s.status,s.amount,s.currency,s.billing_period,s.started_at,s.ends_at,s.trial_ends_at,s.is_trial FROM subscriptions s LEFT JOIN website_pricing_plans p ON p.id=s.plan_id WHERE s.user_id=$1 ORDER BY s.created_at DESC`, userID)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not load subscriptions"})
		return
	}
	subs := []map[string]any{}
	for rows.Next() {
		var id, planID int64
		var publicID, name, status, currency, period string
		var amount float64
		var start time.Time
		var end *time.Time
		var trialEnd *time.Time
		var isTrial bool
		if rows.Scan(&id, &publicID, &planID, &name, &status, &amount, &currency, &period, &start, &end, &trialEnd, &isTrial) != nil {
			rows.Close()
			jsonResponse(w, 500, map[string]string{"error": "Could not read subscriptions"})
			return
		}
		subs = append(subs, map[string]any{"id": id, "public_id": publicID, "plan_id": planID, "plan_name": name, "status": status, "amount": amount, "currency": currency, "billing_period": period, "starts_at": start, "ends_at": end, "trial_ends_at": trialEnd, "is_trial": isTrial})
	}
	rows.Close()
	if rows.Err() != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not read subscriptions"})
		return
	}
	payRows, err := h.DB.Query(r.Context(), `SELECT p.public_id::text,p.plan_id,COALESCE(pl.name,''),p.amount,p.currency,p.provider,p.provider_order_id,p.payment_method,p.status,p.provider_payment_id,p.paid_at,p.created_at FROM payments p LEFT JOIN website_pricing_plans pl ON pl.id=p.plan_id WHERE p.user_id=$1 ORDER BY p.created_at DESC LIMIT 500`, userID)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not load payment history"})
		return
	}
	paymentsOut := []map[string]any{}
	for payRows.Next() {
		var id, name, currency, provider, status string
		var planID int64
		var amount float64
		var orderID, method, paymentID *string
		var paidAt *time.Time
		var created time.Time
		if payRows.Scan(&id, &planID, &name, &amount, &currency, &provider, &orderID, &method, &status, &paymentID, &paidAt, &created) != nil {
			payRows.Close()
			jsonResponse(w, 500, map[string]string{"error": "Could not read payment history"})
			return
		}
		paymentsOut = append(paymentsOut, map[string]any{"payment_public_id": id, "plan_id": planID, "plan_name": name, "amount": amount, "currency": currency, "provider": provider, "provider_order_id": orderID, "provider_payment_id": paymentID, "payment_method": method, "status": status, "transaction_reference": paymentID, "paid_at": paidAt, "created_at": created})
	}
	payRows.Close()
	if payRows.Err() != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not read payment history"})
		return
	}
	jsonResponse(w, 200, map[string]any{"current_subscription": firstActive(subs), "subscriptions": subs, "payments": paymentsOut})
}

// UserPaymentDetails deliberately scopes by both the public payment reference
// and the authenticated customer ID. Another customer's payment is indistinguishable from
// a missing one to the caller.
func (h *Handler) UserPaymentDetails(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUserID(r)
	if !ok {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "User authentication required"})
		return
	}
	ref := strings.TrimSpace(chi.URLParam(r, "id"))
	var publicID, customerName, customerEmail, planName, currency, provider, status, subscriptionStatus string
	var orderID, paymentID, method *string
	var paidAt *time.Time
	var created time.Time
	var amount float64
	var subStarted, subEnds *time.Time
	err := h.DB.QueryRow(r.Context(), `SELECT p.public_id::text,COALESCE(u.name,''),COALESCE(u.email,''),COALESCE(pl.name,''),p.amount,p.currency,p.provider,p.provider_order_id,p.provider_payment_id,p.payment_method,p.status,p.paid_at,p.created_at,COALESCE(s.status,''),s.started_at,s.ends_at FROM payments p JOIN users u ON u.id=p.user_id LEFT JOIN website_pricing_plans pl ON pl.id=p.plan_id LEFT JOIN subscriptions s ON s.id=p.subscription_id WHERE p.public_id::text=$1 AND p.user_id=$2`, ref, userID).Scan(&publicID, &customerName, &customerEmail, &planName, &amount, &currency, &provider, &orderID, &paymentID, &method, &status, &paidAt, &created, &subscriptionStatus, &subStarted, &subEnds)
	if err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Payment not found"})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"data": map[string]any{"payment_public_id": publicID, "customer_name": customerName, "customer_email": customerEmail, "plan_name": planName, "amount": amount, "currency": currency, "provider": provider, "provider_order_id": orderID, "provider_payment_id": paymentID, "payment_method": method, "status": status, "paid_at": paidAt, "created_at": created, "subscription": map[string]any{"status": subscriptionStatus, "started_at": subStarted, "ends_at": subEnds}}})
}

func firstActive(items []map[string]any) any {
	for _, item := range items {
		if item["status"] == "active" || item["status"] == "trial" || item["status"] == "past_due" {
			if end, ok := item["ends_at"].(*time.Time); ok && end != nil && !end.After(time.Now()) {
				continue
			}
			if item["status"] == "trial" {
				if end, ok := item["trial_ends_at"].(*time.Time); ok && end != nil && !end.After(time.Now()) {
					continue
				}
			}
			return item
		}
	}
	return nil
}

func activateAndRecord(ctx context.Context, tx pgx.Tx, paymentID, userID int64, orgID *int64, planID int64, order, amountText, currency string, p razorpay.Payment, eventType, eventID string, clean []byte) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text,0))`, strconv.FormatInt(userID, 10)); err != nil {
		return err
	}
	var period string
	var enabled bool
	if err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT billing_period FROM payments WHERE id=$2),billing_period),is_enabled FROM website_pricing_plans WHERE id=$1`, planID, paymentID).Scan(&period, &enabled); err != nil || !enabled {
		return errors.New("plan unavailable")
	}
	now := time.Now().UTC()
	end := subscriptionPeriodEnd(now, period)
	if end == nil && period != "lifetime" {
		return errors.New("unsupported plan period")
	}
	var subID int64
	if _, err := tx.Exec(ctx, `UPDATE subscriptions SET status='cancelled',updated_at=NOW() WHERE user_id=$1 AND status IN ('active','trial','past_due')`, userID); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `INSERT INTO subscriptions(user_id,organization_id,plan_id,status,started_at,ends_at,renews_at,amount,currency,billing_period) VALUES($1,$2,$3,'active',$4,$5,$5,$6,$7,$8) RETURNING id`, userID, orgID, planID, now, end, amountText, currency, period).Scan(&subID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE payments SET status='paid',error_code=NULL,error_description=NULL,subscription_id=$1,provider_payment_id=$2,payment_method=NULLIF($3,''),bank=NULLIF($4,''),wallet=NULLIF($5,''),paid_at=COALESCE(paid_at,NOW()),updated_at=NOW() WHERE id=$6`, subID, p.ID, safeMethod(p.Method), p.Bank, p.Wallet, paymentID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO payment_transactions(payment_id,provider_event_id,event_type,amount,currency,payload) VALUES($1,NULLIF($2,''),$3,$4,$5,$6::jsonb) ON CONFLICT(provider_event_id) WHERE provider_event_id IS NOT NULL DO NOTHING`, paymentID, eventID, eventType, amountText, currency, string(clean))
	return err
}

func quoteJSON(q pricing.Quote) string { b, _ := json.Marshal(q); return string(b) }
