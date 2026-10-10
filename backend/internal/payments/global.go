package payments

import (
	"context"
	"digital-card-platform/backend/internal/currency"
	"digital-card-platform/backend/internal/pricing"
	_ "embed"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type Method struct {
	ID         string   `json:"id"`
	Countries  []string `json:"countries"`
	Currencies []string `json:"currencies"`
}
type Capability struct {
	Currencies   []string         `json:"currencies"`
	Countries    []string         `json:"countries"`
	MinimumMinor map[string]int64 `json:"minimum_minor"`
	Methods      []Method         `json:"methods"`
	// Manual renewal is the implemented flow. No mandates are collected.
	Renewal string `json:"renewal"`
}
type GlobalConfig struct {
	DisabledCurrencies []string              `json:"disabled_currencies"`
	Capabilities       map[string]Capability `json:"capabilities"`
	MerchantConfirmed  bool                  `json:"merchant_confirmed"`
}

func (h *Handler) globalConfig(ctx context.Context) (GlobalConfig, error) {
	c := GlobalConfig{DisabledCurrencies: []string{}, Capabilities: map[string]Capability{}}
	if h.DB == nil {
		return c, nil
	}
	var raw string
	err := h.DB.QueryRow(ctx, `SELECT setting_value FROM platform_settings WHERE setting_key=$1`, "billing_global_"+strings.ToLower(strings.TrimSpace(os.Getenv("RAZORPAY_MODE")))).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	err = json.Unmarshal([]byte(raw), &c)
	normalizeGlobal(&c)
	return c, err
}
func (c GlobalConfig) Active(code string) bool {
	for _, x := range c.DisabledCurrencies {
		if x == currency.Normalize(code) {
			return false
		}
	}
	return currency.Supported(code)
}
func contains(a []string, s string) bool {
	for _, v := range a {
		if v == s || v == "*" {
			return true
		}
	}
	return false
}
func (c GlobalConfig) Check(provider, code, country string, minor int64) ([]string, error) {
	if !c.Active(code) {
		return nil, errors.New("Selected currency is inactive")
	}
	cap, ok := c.Capabilities[provider]
	if !ok {
		return nil, nil
	} // Native Checkout remains authoritative about methods.
	if !c.MerchantConfirmed {
		return nil, errors.New("Provider capabilities need merchant confirmation")
	}
	if len(cap.Currencies) > 0 && !contains(cap.Currencies, code) {
		return nil, errors.New("Payment provider is not enabled for the selected currency")
	}
	if len(cap.Countries) > 0 && !contains(cap.Countries, country) {
		return nil, errors.New("Payment provider is not enabled for the selected country")
	}
	if minimum := cap.MinimumMinor[code]; minimum > 0 && minor < minimum {
		return nil, errors.New("Selected amount is below the provider minimum")
	}
	methods := []string{}
	for _, m := range cap.Methods {
		if (len(m.Countries) == 0 || contains(m.Countries, country)) && (len(m.Currencies) == 0 || contains(m.Currencies, code)) {
			methods = append(methods, m.ID)
		}
	}
	if len(cap.Methods) > 0 && len(methods) == 0 {
		return nil, errors.New("No configured payment method supports this country and currency")
	}
	return methods, nil
}
func validateGlobal(c GlobalConfig) error {
	for _, code := range c.DisabledCurrencies {
		if !currency.Supported(code) || code != currency.Normalize(code) {
			return errors.New("Invalid inactive currency")
		}
	}
	for provider, cap := range c.Capabilities {
		if provider == "" || len(provider) > 30 {
			return errors.New("Invalid provider identifier")
		}
		for _, code := range cap.Currencies {
			if !currency.Supported(code) || code != currency.Normalize(code) || (provider == "razorpay" && !publishedRazorpayCurrency(code)) {
				return errors.New("Invalid provider currency")
			}
		}
		if provider == "razorpay" {
			for code := range cap.MinimumMinor {
				if !publishedRazorpayCurrency(code) {
					return errors.New("Currency is not in the provider support registry")
				}
			}
		}
		if cap.Renewal != "" && cap.Renewal != "manual" {
			return errors.New("Only manual renewal is implemented")
		}
		for _, country := range cap.Countries {
			if country != "*" {
				if normalized, err := pricing.Country(country); err != nil || normalized != country {
					return errors.New("Invalid provider country")
				}
			}
		}
		for code, min := range cap.MinimumMinor {
			if !currency.Supported(code) || min < 1 {
				return errors.New("Invalid provider minimum")
			}
		}
		for _, method := range cap.Methods {
			if !contains([]string{"card", "upi", "netbanking", "wallet", "emi", "paylater", "bank_transfer"}, method.ID) {
				return errors.New("Unknown payment method")
			}
			for _, country := range method.Countries {
				if country != "*" {
					if normalized, err := pricing.Country(country); err != nil || normalized != country {
						return errors.New("Invalid method country")
					}
				}
			}
			for _, code := range method.Currencies {
				if code != "*" && !currency.Supported(code) {
					return errors.New("Invalid method currency")
				}
			}
			if provider == "razorpay" && method.ID == "bank_transfer" {
				return errors.New("Bank transfer requires a separately approved Razorpay flow")
			}
			if provider == "razorpay" && method.ID != "card" { // Methods on standard non-INR Checkout must not be invented.
				if len(method.Currencies) != 1 || method.Currencies[0] != "INR" || len(method.Countries) != 1 || method.Countries[0] != "IN" {
					return errors.New("Razorpay non-card methods require explicit INR / IN scope")
				}
			}
		}
	}
	if len(c.Capabilities) > 0 && !c.MerchantConfirmed {
		return errors.New("Confirm capabilities in the merchant dashboard before saving")
	}
	return nil
}
func (h *Handler) AdminGlobalBilling(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		c, err := h.globalConfig(r.Context())
		if err != nil {
			jsonResponse(w, 503, map[string]string{"error": "Could not load billing configuration"})
			return
		}
		jsonResponse(w, 200, map[string]any{"data": c})
		return
	}
	var c GlobalConfig
	d := json.NewDecoder(io.LimitReader(r.Body, 128<<10))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil || d.Decode(new(any)) != io.EOF {
		jsonResponse(w, 400, map[string]string{"error": "Invalid billing configuration"})
		return
	}
	normalizeGlobal(&c)
	if err := validateGlobal(c); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not save billing configuration"})
		return
	}
	defer tx.Rollback(r.Context())
	raw, _ := json.Marshal(c)
	_, err = tx.Exec(r.Context(), `INSERT INTO platform_settings(setting_key,setting_value,updated_at) VALUES($1,$2,NOW()) ON CONFLICT(setting_key) DO UPDATE SET setting_value=EXCLUDED.setting_value,updated_at=NOW()`, "billing_global_"+strings.ToLower(strings.TrimSpace(os.Getenv("RAZORPAY_MODE"))), string(raw))
	if err != nil || tx.Commit(r.Context()) != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not save billing configuration"})
		return
	}
	jsonResponse(w, 200, map[string]any{"data": c})
}
func (h *Handler) PlanPrices(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		jsonResponse(w, 400, map[string]string{"error": "Invalid plan"})
		return
	}
	if r.Method == "GET" {
		var raw []byte
		err := h.DB.QueryRow(r.Context(), `SELECT billing_config FROM website_pricing_plans WHERE id=$1 AND archived_at IS NULL`, id).Scan(&raw)
		if err != nil {
			jsonResponse(w, 404, map[string]string{"error": "Plan not found"})
			return
		}
		jsonResponse(w, 200, map[string]any{"data": json.RawMessage(raw)})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 128<<10))
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": "Invalid price configuration"})
		return
	}
	cfg, err := pricing.Parse(raw)
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not save prices"})
		return
	}
	defer tx.Rollback(r.Context())
	raw, _ = json.Marshal(cfg)
	tag, err := tx.Exec(r.Context(), `UPDATE website_pricing_plans SET billing_config=$1::jsonb,updated_at=NOW() WHERE id=$2 AND archived_at IS NULL`, string(raw), id)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not save prices"})
		return
	}
	if tag.RowsAffected() != 1 {
		jsonResponse(w, 404, map[string]string{"error": "Plan not found"})
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Could not save prices"})
		return
	}
	jsonResponse(w, 200, map[string]any{"data": cfg})
}

// Location headers are trusted only behind an explicitly configured proxy that
// strips client-supplied values. They suggest a currency; never select a charge.
func (h *Handler) CurrencyCatalog(w http.ResponseWriter, r *http.Request) {
	c, err := h.globalConfig(r.Context())
	if err != nil {
		jsonResponse(w, 503, map[string]string{"error": "Could not load currencies"})
		return
	}
	list := []currency.Currency{}
	for _, item := range currency.Catalog() {
		if c.Active(item.Code) {
			list = append(list, item)
		}
	}
	country := ""
	if os.Getenv("BILLING_TRUST_LOCATION_HEADERS") == "true" {
		country, _ = pricing.Country(r.Header.Get("CF-IPCountry"))
	}
	var suggestions map[string]string
	_ = json.Unmarshal(countryCurrenciesJSON, &suggestions)
	suggested := suggestions[country]
	if !c.Active(suggested) {
		suggested = ""
	}
	jsonResponse(w, 200, map[string]any{"data": list, "country": country, "suggested_currency": suggested})
}

// Snapshot of current tender currencies from Unicode CLDR supplemental currencyData.json.
//
//go:embed country-currencies.json
var countryCurrenciesJSON []byte

func normalizeGlobal(c *GlobalConfig) {
	if c.DisabledCurrencies == nil {
		c.DisabledCurrencies = []string{}
	}
	if c.Capabilities == nil {
		c.Capabilities = map[string]Capability{}
	}
	for provider, cap := range c.Capabilities {
		if cap.Countries == nil {
			cap.Countries = []string{}
		}
		if cap.Currencies == nil {
			cap.Currencies = []string{}
		}
		if cap.MinimumMinor == nil {
			cap.MinimumMinor = map[string]int64{}
		}
		if cap.Methods == nil {
			cap.Methods = []Method{}
		}
		if cap.Renewal == "" {
			cap.Renewal = "manual"
		}
		c.Capabilities[provider] = cap
	}
}
