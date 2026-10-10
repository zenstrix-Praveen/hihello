package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"digital-card-platform/backend/internal/currency"
	"digital-card-platform/backend/internal/customer"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PlansBillingHandler struct{ DB *pgxpool.Pool }

func (h *PlansBillingHandler) FeatureRegistry(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"data": customer.FeatureRegistry})
}

func (h *PlansBillingHandler) GetPlanEntitlements(w http.ResponseWriter, r *http.Request) {
	id, err := routeID(r)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid plan id"})
		return
	}
	var features, limits []byte
	err = h.DB.QueryRow(r.Context(), `SELECT features,limits FROM website_pricing_plans WHERE id=$1`, id).Scan(&features, &limits)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, 404, map[string]string{"error": "Plan not found"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load plan entitlements"})
		return
	}
	resolved, err := customer.NormalizePlanEntitlements(features, limits)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load plan entitlements"})
		return
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"plan_id": id, "entitlements": resolved}})
}

func (h *PlansBillingHandler) SavePlanEntitlements(w http.ResponseWriter, r *http.Request) {
	id, err := routeID(r)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid plan id"})
		return
	}
	var in struct {
		Entitlements map[string]customer.Entitlement `json:"entitlements"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	dec.DisallowUnknownFields()
	if dec.Decode(&in) != nil || in.Entitlements == nil || dec.Decode(new(any)) != io.EOF {
		writeJSON(w, 400, map[string]string{"error": "Invalid plan entitlements"})
		return
	}
	allowed := make(map[string]customer.Feature, len(customer.FeatureRegistry))
	for _, feature := range customer.FeatureRegistry {
		allowed[feature.Code] = feature
	}
	if len(in.Entitlements) != len(allowed) {
		writeJSON(w, 400, map[string]string{"error": "Provide an entitlement value for each registered feature"})
		return
	}
	for code := range in.Entitlements {
		if _, ok := allowed[code]; !ok {
			writeJSON(w, 400, map[string]string{"error": "Unknown feature code"})
			return
		}
	}
	features := make([]string, 0, len(in.Entitlements))
	for _, feature := range customer.FeatureRegistry {
		value := in.Entitlements[feature.Code]
		if feature.Type == "numeric" {
			if value.Limit < -1 || value.Enabled != (value.Limit != 0) {
				writeJSON(w, 400, map[string]string{"error": "Numeric limits must be -1, 0, or a positive whole number and match enabled state"})
				return
			}
		} else if value.Limit != 0 {
			writeJSON(w, 400, map[string]string{"error": "Boolean entitlements cannot have numeric limits"})
			return
		}
		if feature.Type == "boolean" && value.Enabled {
			features = append(features, feature.Code)
		}
	}
	var oldFeatures, oldLimits []byte
	err = h.DB.QueryRow(r.Context(), `SELECT features,limits FROM website_pricing_plans WHERE id=$1`, id).Scan(&oldFeatures, &oldLimits)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, 404, map[string]string{"error": "Plan not found"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not save plan entitlements"})
		return
	}
	var oldCodes []string
	if json.Unmarshal(oldFeatures, &oldCodes) != nil {
		oldCodes = []string{}
	}
	knownBoolean := map[string]bool{}
	for _, f := range customer.FeatureRegistry {
		if f.Type == "boolean" {
			knownBoolean[f.Code] = true
		}
	}
	for _, old := range oldCodes {
		if !knownBoolean[old] {
			features = append(features, old)
		}
	}
	limits := map[string]json.RawMessage{}
	if json.Unmarshal(oldLimits, &limits) != nil || limits == nil {
		limits = map[string]json.RawMessage{}
	}
	for _, feature := range customer.FeatureRegistry {
		if feature.Type != "numeric" {
			continue
		}
		unlimited := in.Entitlements[feature.Code].Limit == -1
		limit := in.Entitlements[feature.Code].Limit
		if unlimited {
			limit = 0
		}
		raw, _ := json.Marshal(limit)
		limits[feature.Code+"_limit"] = raw
		unlimitedRaw, _ := json.Marshal(unlimited)
		limits[feature.Code+"_unlimited"] = unlimitedRaw
	}
	featureJSON, _ := json.Marshal(features)
	limitJSON, _ := json.Marshal(limits)
	tag, err := h.DB.Exec(r.Context(), `UPDATE website_pricing_plans SET features=$1,limits=$2,updated_at=NOW() WHERE id=$3`, featureJSON, limitJSON, id)
	if err != nil || tag.RowsAffected() != 1 {
		writeJSON(w, 500, map[string]string{"error": "Could not save plan entitlements"})
		return
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"plan_id": id, "entitlements": in.Entitlements}})
}

type planInput struct {
	Name          string          `json:"name"`
	Code          string          `json:"code"`
	Description   string          `json:"description"`
	Price         float64         `json:"price"`
	PricingType   string          `json:"pricing_type"`
	Currency      string          `json:"currency"`
	BillingPeriod string          `json:"billing_period"`
	TrialDays     int             `json:"trial_days"`
	Features      []string        `json:"features"`
	Limits        json.RawMessage `json:"limits"`
	Featured      bool            `json:"is_featured"`
	Active        bool            `json:"is_active"`
	SortOrder     int             `json:"sort_order"`
	Visibility    string          `json:"visibility"`
	MostPopular   bool            `json:"is_most_popular"`
	BillingType   string          `json:"billing_type"`
	TrialMode     string          `json:"trial_mode"`
}
type subscriptionInput struct {
	PlanID         int64    `json:"plan_id"`
	UserID         *int64   `json:"user_id"`
	OrganizationID *int64   `json:"organization_id"`
	Status         string   `json:"status"`
	StartsAt       string   `json:"starts_at"`
	EndsAt         string   `json:"ends_at"`
	TrialEndsAt    string   `json:"trial_ends_at"`
	Amount         *float64 `json:"amount"`
	Currency       string   `json:"currency"`
	BillingPeriod  string   `json:"billing_period"`
}

var planCodePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,99}$`)
var billingPeriods = map[string]bool{"monthly": true, "yearly": true, "lifetime": true, "custom": true}
var subscriptionStatuses = map[string]bool{"trial": true, "active": true, "past_due": true, "cancelled": true, "expired": true}

func (h *PlansBillingHandler) Summary(w http.ResponseWriter, r *http.Request) {
	var out struct {
		TotalPlans          int64              `json:"total_plans"`
		ActivePlans         int64              `json:"active_plans"`
		TotalSubscribers    int64              `json:"total_subscribers"`
		ActiveSubscriptions int64              `json:"active_subscriptions"`
		TrialSubscriptions  int64              `json:"trial_subscriptions"`
		MonthlyRevenue      map[string]float64 `json:"monthly_revenue"`
		AnnualRevenue       map[string]float64 `json:"annual_revenue"`
	}
	err := h.DB.QueryRow(r.Context(), `SELECT (SELECT COUNT(*) FROM website_pricing_plans WHERE archived_at IS NULL),(SELECT COUNT(*) FROM website_pricing_plans WHERE is_enabled AND archived_at IS NULL),(SELECT COUNT(DISTINCT COALESCE('u:'||user_id::text,'o:'||organization_id::text,'s:'||id::text)) FROM subscriptions),(SELECT COUNT(*) FROM subscriptions WHERE status='active'),(SELECT COUNT(*) FROM subscriptions WHERE status='trial')`).Scan(&out.TotalPlans, &out.ActivePlans, &out.TotalSubscribers, &out.ActiveSubscriptions, &out.TrialSubscriptions)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load billing summary"})
		return
	}
	out.MonthlyRevenue, out.AnnualRevenue = map[string]float64{}, map[string]float64{}
	rows, err := h.DB.Query(r.Context(), `SELECT currency,COALESCE(SUM(CASE WHEN billing_period='monthly' THEN amount WHEN billing_period='yearly' THEN amount/12 ELSE 0 END),0),COALESCE(SUM(CASE WHEN billing_period='monthly' THEN amount*12 WHEN billing_period='yearly' THEN amount ELSE 0 END),0) FROM subscriptions WHERE status='active' GROUP BY currency`)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load revenue summary"})
		return
	}
	defer rows.Close()
	for rows.Next() {
		var currency string
		var monthly, annual float64
		if err := rows.Scan(&currency, &monthly, &annual); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read revenue summary"})
			return
		}
		out.MonthlyRevenue[currency], out.AnnualRevenue[currency] = monthly, annual
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not read revenue summary"})
		return
	}
	writeJSON(w, 200, out)
}

func (h *PlansBillingHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	q := `SELECT p.id,p.public_id::text,p.name,p.code,p.description,p.price,p.pricing_type,p.currency,p.billing_period,p.trial_days,p.features,p.limits,p.is_featured,p.is_enabled,p.sort_order,p.created_at,p.updated_at,(SELECT COUNT(DISTINCT COALESCE('u:'||s.user_id::text,'o:'||s.organization_id::text,'s:'||s.id::text)) FROM subscriptions s WHERE s.plan_id=p.id),EXISTS(SELECT 1 FROM subscriptions s WHERE s.plan_id=p.id) OR EXISTS(SELECT 1 FROM payments pm WHERE pm.plan_id=p.id),p.visibility,p.is_most_popular,p.billing_type,p.trial_mode FROM website_pricing_plans p WHERE p.archived_at IS NULL AND ($1='' OR p.name ILIKE $2 OR p.code ILIKE $2) AND ($3='' OR p.is_enabled=($3='active')) ORDER BY p.sort_order,p.id`
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	rows, err := h.DB.Query(r.Context(), q, search, "%"+search+"%", r.URL.Query().Get("status"))
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load plans"})
		return
	}
	defer rows.Close()
	items := []map[string]interface{}{}
	for rows.Next() {
		var id int64
		var pid, name, code, desc, pricingType, currency, period string
		var visibility, billingType, trialMode string
		var price float64
		var trial, order int
		var features, limits []byte
		var featured, active, popular bool
		var created, updated time.Time
		var count int64
		var hasHistory bool
		if err = rows.Scan(&id, &pid, &name, &code, &desc, &price, &pricingType, &currency, &period, &trial, &features, &limits, &featured, &active, &order, &created, &updated, &count, &hasHistory, &visibility, &popular, &billingType, &trialMode); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read plans"})
			return
		}
		items = append(items, map[string]interface{}{"id": id, "public_id": pid, "name": name, "code": code, "description": desc, "price": price, "pricing_type": pricingType, "currency": currency, "billing_period": period, "trial_days": trial, "features": json.RawMessage(features), "limits": json.RawMessage(limits), "is_featured": featured, "is_active": active, "sort_order": order, "created_at": created, "updated_at": updated, "subscriber_count": count, "has_history": hasHistory, "visibility": visibility, "is_most_popular": popular, "billing_type": billingType, "trial_mode": trialMode})
	}
	if err = rows.Err(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not read plans"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": items})
}
func (h *PlansBillingHandler) GetPlan(w http.ResponseWriter, r *http.Request) {
	id, e := routeID(r)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid plan id"})
		return
	}
	p, e := h.plan(r, id)
	if e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			writeJSON(w, 404, map[string]string{"error": "Plan not found"})
		} else {
			writeJSON(w, 500, map[string]string{"error": "Could not load plan"})
		}
		return
	}
	rows, e := h.DB.Query(r.Context(), `SELECT s.id,s.status,s.amount,s.currency,s.billing_period,s.started_at,s.renews_at,s.user_id,s.organization_id,COALESCE(u.name,''),COALESCE(u.email,''),COALESCE(o.name,'') FROM subscriptions s LEFT JOIN users u ON u.id=s.user_id LEFT JOIN organizations o ON o.id=s.organization_id WHERE s.plan_id=$1 ORDER BY s.created_at DESC LIMIT 200`, id)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load subscribers"})
		return
	}
	defer rows.Close()
	subs := []map[string]interface{}{}
	for rows.Next() {
		var sid int64
		var status, currency, period, un, ue, on string
		var amt float64
		var start time.Time
		var renew *time.Time
		var uid, oid *int64
		if rows.Scan(&sid, &status, &amt, &currency, &period, &start, &renew, &uid, &oid, &un, &ue, &on) != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read subscribers"})
			return
		}
		subs = append(subs, map[string]interface{}{"id": sid, "status": status, "amount": amt, "currency": currency, "billing_period": period, "starts_at": start, "ends_at": renew, "user_id": uid, "organization_id": oid, "user_name": un, "user_email": ue, "organization_name": on})
	}
	p["subscribers"] = subs
	writeJSON(w, 200, p)
}
func (h *PlansBillingHandler) plan(r *http.Request, id int64) (map[string]interface{}, error) {
	var pid, name, code, desc, pricingType, currency, period, visibility, billingType, trialMode string
	var price float64
	var trial, order int
	var f, l []byte
	var featured, active, popular bool
	var created, updated time.Time
	var count int64
	e := h.DB.QueryRow(r.Context(), `SELECT p.public_id::text,p.name,p.code,p.description,p.price,p.pricing_type,p.currency,p.billing_period,p.trial_days,p.features,p.limits,p.is_featured,p.is_enabled,p.sort_order,p.created_at,p.updated_at,(SELECT COUNT(DISTINCT COALESCE('u:'||s.user_id::text,'o:'||s.organization_id::text,'s:'||s.id::text)) FROM subscriptions s WHERE s.plan_id=p.id),p.visibility,p.is_most_popular,p.billing_type,p.trial_mode FROM website_pricing_plans p WHERE p.id=$1`, id).Scan(&pid, &name, &code, &desc, &price, &pricingType, &currency, &period, &trial, &f, &l, &featured, &active, &order, &created, &updated, &count, &visibility, &popular, &billingType, &trialMode)
	if e != nil {
		return nil, e
	}
	return map[string]interface{}{"id": id, "public_id": pid, "name": name, "code": code, "description": desc, "price": price, "pricing_type": pricingType, "currency": currency, "billing_period": period, "trial_days": trial, "features": json.RawMessage(f), "limits": json.RawMessage(l), "is_featured": featured, "is_active": active, "sort_order": order, "created_at": created, "updated_at": updated, "subscriber_count": count, "visibility": visibility, "is_most_popular": popular, "billing_type": billingType, "trial_mode": trialMode}, nil
}

func normalizePlan(p *planInput) error {
	return normalizePlanWithExistingCurrency(p, "")
}

// Only the stored currency may bypass the selectable catalog on an edit.
// This keeps legacy plans editable without introducing new unsupported codes.
func normalizePlanWithExistingCurrency(p *planInput, existingCurrency string) error {
	p.Name = strings.TrimSpace(p.Name)
	p.Code = strings.ToLower(strings.TrimSpace(p.Code))
	p.Currency = strings.ToUpper(strings.TrimSpace(p.Currency))
	p.BillingPeriod = strings.ToLower(strings.TrimSpace(p.BillingPeriod))
	if p.Name == "" || !planCodePattern.MatchString(p.Code) {
		return errors.New("Plan name and a valid unique code are required")
	}
	if math.IsNaN(p.Price) || math.IsInf(p.Price, 0) || p.Price < 0 {
		return errors.New("Price must be non-negative")
	}
	if len(p.Currency) != 3 || !asciiLetters(p.Currency) {
		return errors.New("Currency must be a three-letter code")
	}
	if !currency.Supported(p.Currency) && (existingCurrency == "" || p.Currency != existingCurrency) {
		return errors.New("Select a supported plan currency")
	}
	if exponent, ok := currency.Exponent(p.Currency); ok {
		priceText := strconv.FormatFloat(p.Price, 'f', -1, 64)
		fraction := ""
		if dot := strings.IndexByte(priceText, '.'); dot >= 0 {
			fraction = strings.TrimRight(priceText[dot+1:], "0")
		}
		if len(fraction) > exponent {
			return fmt.Errorf("Price supports at most %d decimal places for %s", exponent, p.Currency)
		}
	}
	if !billingPeriods[p.BillingPeriod] {
		return errors.New("Invalid billing period")
	}
	p.PricingType = strings.ToLower(strings.TrimSpace(p.PricingType))
	if p.PricingType == "" {
		if p.BillingPeriod == "custom" {
			p.PricingType = "custom"
		} else if p.Price == 0 {
			p.PricingType = "free"
		} else {
			p.PricingType = "fixed"
		}
	}
	switch p.PricingType {
	case "free":
		if p.Price != 0 || p.BillingPeriod == "custom" {
			return errors.New("Free plans must have a zero price and a non-custom billing period")
		}
	case "fixed":
		if p.Price <= 0 || p.BillingPeriod == "custom" {
			return errors.New("Fixed-price plans require a positive price and a purchasable billing period")
		}
	case "custom":
		p.Price = 0
		p.BillingPeriod = "custom"
	default:
		return errors.New("Pricing type must be free, fixed, or custom")
	}
	if p.TrialDays < 0 {
		return errors.New("Trial days cannot be negative")
	}
	if p.Visibility == "" {
		p.Visibility = "public"
	}
	if p.Visibility != "public" && p.Visibility != "hidden" {
		return errors.New("Visibility must be public or hidden")
	}
	if p.BillingType == "" {
		p.BillingType = "per_user"
	}
	if p.BillingType != "per_user" && p.BillingType != "per_workspace" {
		return errors.New("Billing type must be per_user or per_workspace")
	}
	if p.TrialMode == "" {
		if p.TrialDays > 0 {
			p.TrialMode = "limited"
		} else {
			p.TrialMode = "disabled"
		}
	}
	if p.TrialMode != "disabled" && p.TrialMode != "limited" && p.TrialMode != "unlimited" {
		return errors.New("Trial mode must be disabled, limited, or unlimited")
	}
	if p.TrialMode == "unlimited" {
		p.TrialDays = 0
	}
	if p.TrialMode == "disabled" {
		p.TrialDays = 0
	}
	if p.TrialMode == "limited" && p.TrialDays < 1 {
		return errors.New("Limited trials require at least one trial day")
	}
	if p.Features == nil {
		p.Features = []string{}
	}
	if len(p.Limits) == 0 {
		p.Limits = json.RawMessage(`{}`)
	}
	var limits map[string]interface{}
	if json.Unmarshal(p.Limits, &limits) != nil || limits == nil {
		return errors.New("Limits must be a JSON object")
	}
	for _, key := range []string{"digital_cards_limit", "qr_codes_limit", "nfc_tags_limit", "contacts_limit", "team_members_limit", "storage_limit", "card_templates_limit"} {
		value, exists := limits[key]
		if !exists {
			continue
		}
		number, ok := value.(float64)
		if !ok || math.IsNaN(number) || math.IsInf(number, 0) || math.Trunc(number) != number || number < 0 || number > 2147483647 {
			return errors.New("Numeric feature limits must be non-negative whole numbers; use the explicit unlimited setting")
		}
		if marker, ok := limits[strings.TrimSuffix(key, "_limit")+"_unlimited"]; ok {
			if _, valid := marker.(bool); !valid {
				return errors.New("Unlimited feature flags must be boolean")
			}
		}
	}
	return nil
}
func asciiLetters(s string) bool {
	for _, c := range s {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}
func (h *PlansBillingHandler) savePlan(w http.ResponseWriter, r *http.Request, id int64) {
	creating := id == 0
	var p planInput
	if json.NewDecoder(r.Body).Decode(&p) != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid plan data"})
		return
	}
	var existingCurrency string
	if id != 0 {
		err := h.DB.QueryRow(r.Context(), `SELECT currency FROM website_pricing_plans WHERE id=$1 AND archived_at IS NULL`, id).Scan(&existingCurrency)
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, 404, map[string]string{"error": "Plan not found"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not load plan currency"})
			return
		}
	}
	if e := normalizePlanWithExistingCurrency(&p, existingCurrency); e != nil {
		writeJSON(w, 400, map[string]string{"error": e.Error()})
		return
	}
	features, _ := json.Marshal(p.Features)
	var err error
	if id == 0 {
		err = h.DB.QueryRow(r.Context(), `INSERT INTO website_pricing_plans(name,code,description,price,pricing_type,currency,billing_period,trial_days,features,limits,is_featured,is_enabled,sort_order,visibility,is_most_popular,billing_type,trial_mode) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17) RETURNING id`, p.Name, p.Code, p.Description, p.Price, p.PricingType, p.Currency, p.BillingPeriod, p.TrialDays, features, p.Limits, p.Featured, p.Active, p.SortOrder, p.Visibility, p.MostPopular, p.BillingType, p.TrialMode).Scan(&id)
	} else {
		var tag pgconn.CommandTag
		tag, err = h.DB.Exec(r.Context(), `UPDATE website_pricing_plans SET name=$1,code=$2,description=$3,price=$4,pricing_type=$5,currency=$6,billing_period=$7,trial_days=$8,features=$9,limits=$10,is_featured=$11,is_enabled=$12,sort_order=$13,visibility=$14,is_most_popular=$15,billing_type=$16,trial_mode=$17,updated_at=NOW() WHERE id=$18 AND archived_at IS NULL`, p.Name, p.Code, p.Description, p.Price, p.PricingType, p.Currency, p.BillingPeriod, p.TrialDays, features, p.Limits, p.Featured, p.Active, p.SortOrder, p.Visibility, p.MostPopular, p.BillingType, p.TrialMode, id)
		if err == nil && tag.RowsAffected() == 0 {
			writeJSON(w, 404, map[string]string{"error": "Plan not found"})
			return
		}
	}
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			writeJSON(w, 409, map[string]string{"error": "Plan code already exists"})
			return
		}
		writeJSON(w, 500, map[string]string{"error": "Could not save plan"})
		return
	}
	out, e := h.plan(r, id)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Plan saved but could not be loaded"})
		return
	}
	if creating {
		auditAdmin(r, h.DB, "plan_created", "plan", id, "Created subscription plan")
	} else {
		auditAdmin(r, h.DB, "plan_updated", "plan", id, "Updated subscription plan")
	}
	writeJSON(w, 200, out)
}
func (h *PlansBillingHandler) CreatePlan(w http.ResponseWriter, r *http.Request) { h.savePlan(w, r, 0) }
func (h *PlansBillingHandler) UpdatePlan(w http.ResponseWriter, r *http.Request) {
	id, e := routeID(r)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid plan id"})
		return
	}
	h.savePlan(w, r, id)
}
func (h *PlansBillingHandler) PlanStatus(w http.ResponseWriter, r *http.Request) {
	id, e := routeID(r)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid plan id"})
		return
	}
	var in struct {
		Active *bool `json:"is_active"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.Active == nil {
		writeJSON(w, 400, map[string]string{"error": "is_active is required"})
		return
	}
	tag, e := h.DB.Exec(r.Context(), `UPDATE website_pricing_plans SET is_enabled=$1,updated_at=NOW() WHERE id=$2 AND archived_at IS NULL`, *in.Active, id)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not update plan status"})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, 404, map[string]string{"error": "Plan not found"})
		return
	}
	p, e := h.plan(r, id)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load plan"})
		return
	}
	auditAdmin(r, h.DB, "plan_status_changed", "plan", id, "Plan active status changed")
	writeJSON(w, 200, p)
}
func (h *PlansBillingHandler) DeletePlan(w http.ResponseWriter, r *http.Request) {
	id, e := routeID(r)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid plan id"})
		return
	}
	ctx := r.Context()
	tx, e := h.DB.Begin(ctx)
	if e != nil {
		log.Printf("DeletePlan transaction failed: %v", e)
		writeJSON(w, 500, map[string]string{"error": "Could not delete plan"})
		return
	}
	defer tx.Rollback(ctx)
	var lockedID int64
	e = tx.QueryRow(ctx, `SELECT id FROM website_pricing_plans WHERE id=$1 FOR UPDATE`, id).Scan(&lockedID)
	if errors.Is(e, pgx.ErrNoRows) {
		writeJSON(w, 404, map[string]string{"error": "Plan not found"})
		return
	}
	if e != nil {
		log.Printf("DeletePlan lookup failed: %v", e)
		writeJSON(w, 500, map[string]string{"error": "Could not delete plan"})
		return
	}
	tag, e := tx.Exec(ctx, `UPDATE website_pricing_plans SET is_enabled=false,archived_at=COALESCE(archived_at,NOW()),updated_at=NOW() WHERE id=$1`, id)
	if e != nil {
		log.Printf("DeletePlan archive failed: %v", e)
		writeJSON(w, 500, map[string]string{"error": "Could not delete plan"})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, 404, map[string]string{"error": "Plan not found"})
		return
	}
	if e = tx.Commit(ctx); e != nil {
		log.Printf("DeletePlan commit failed: %v", e)
		writeJSON(w, 500, map[string]string{"error": "Could not delete plan"})
		return
	}
	auditAdmin(r, h.DB, "plan_archived", "plan", id, "Archived subscription plan")
	writeJSON(w, 200, map[string]string{"message": "Plan archived"})
}

func (h *PlansBillingHandler) PublicPlans(w http.ResponseWriter, r *http.Request) {
	rows, e := h.DB.Query(r.Context(), `SELECT id,public_id::text,code,name,description,price,pricing_type,currency,billing_period,trial_days,features,limits,cta_text,cta_url,is_featured,is_most_popular,visibility,billing_type,trial_mode FROM website_pricing_plans WHERE is_enabled=true AND visibility='public' AND archived_at IS NULL ORDER BY sort_order,id`)
	if e != nil {
		log.Printf("PublicPlans query failed: %v", e)
		writeJSON(w, 500, map[string]string{"error": "Could not load plans"})
		return
	}
	defer rows.Close()
	a := []map[string]interface{}{}
	for rows.Next() {
		var numericID int64
		var id, code, n, d, pricingType, c, b, ctaText, ctaURL, visibility, billingType, trialMode string
		var price float64
		var days int
		var f, l []byte
		var featured, popular bool
		if err := rows.Scan(&numericID, &id, &code, &n, &d, &price, &pricingType, &c, &b, &days, &f, &l, &ctaText, &ctaURL, &featured, &popular, &visibility, &billingType, &trialMode); err != nil {
			log.Printf("PublicPlans row scan failed: %v", err)
			writeJSON(w, 500, map[string]string{"error": "Could not read plans"})
			return
		}
		a = append(a, map[string]interface{}{"id": numericID, "public_id": id, "code": code, "name": n, "description": d, "price": price, "pricing_type": pricingType, "currency": c, "billing_period": b, "trial_days": days, "features": json.RawMessage(f), "limits": json.RawMessage(l), "cta_text": ctaText, "cta_url": ctaURL, "is_featured": featured, "is_most_popular": popular, "visibility": visibility, "billing_type": billingType, "trial_mode": trialMode})
	}
	if err := rows.Err(); err != nil {
		log.Printf("PublicPlans rows failed: %v", err)
		writeJSON(w, 500, map[string]string{"error": "Could not read plans"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": a})
}

func (h *PlansBillingHandler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	search := "%" + strings.TrimSpace(r.URL.Query().Get("search")) + "%"
	status := r.URL.Query().Get("status")
	pid := r.URL.Query().Get("plan_id")
	rows, e := h.DB.Query(r.Context(), `SELECT s.id,s.public_id::text,s.plan_id,s.user_id,s.organization_id,s.status,s.amount,s.currency,s.billing_period,s.started_at,s.ends_at,s.trial_ends_at,s.created_at,p.name,COALESCE(u.name,''),COALESCE(u.email,''),COALESCE(o.name,'') FROM subscriptions s LEFT JOIN website_pricing_plans p ON p.id=s.plan_id LEFT JOIN users u ON u.id=s.user_id LEFT JOIN organizations o ON o.id=s.organization_id WHERE ($1='' OR s.status=$1) AND ($2='' OR s.plan_id::text=$2) AND ($3='%%' OR COALESCE(u.name,'') ILIKE $3 OR COALESCE(u.email,'') ILIKE $3 OR COALESCE(o.name,'') ILIKE $3) ORDER BY s.created_at DESC LIMIT 500`, status, pid, search)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load subscriptions"})
		return
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		m, e := scanSubscription(rows)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read subscriptions"})
			return
		}
		out = append(out, m)
	}
	writeJSON(w, 200, map[string]interface{}{"data": out})
}
func scanSubscription(rows pgx.Rows) (map[string]interface{}, error) {
	var id int64
	var public string
	var pid, uid, oid *int64
	var status, curr, period, pn, un, ue, on string
	var amount float64
	var start, created time.Time
	var end, trial *time.Time
	e := rows.Scan(&id, &public, &pid, &uid, &oid, &status, &amount, &curr, &period, &start, &end, &trial, &created, &pn, &un, &ue, &on)
	return map[string]interface{}{"id": id, "public_id": public, "plan_id": pid, "user_id": uid, "organization_id": oid, "status": status, "amount": amount, "currency": curr, "billing_period": period, "starts_at": start, "ends_at": end, "trial_ends_at": trial, "created_at": created, "plan_name": pn, "user_name": un, "user_email": ue, "organization_name": on}, e
}
func (h *PlansBillingHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	id, e := routeID(r)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid subscription id"})
		return
	}
	m, e := h.subscription(r, id)
	if e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			writeJSON(w, 404, map[string]string{"error": "Subscription not found"})
		} else {
			writeJSON(w, 500, map[string]string{"error": "Could not load subscription"})
		}
		return
	}
	writeJSON(w, 200, m)
}
func (h *PlansBillingHandler) subscription(r *http.Request, id int64) (map[string]interface{}, error) {
	rows, e := h.DB.Query(r.Context(), `SELECT s.id,s.public_id::text,s.plan_id,s.user_id,s.organization_id,s.status,s.amount,s.currency,s.billing_period,s.started_at,s.ends_at,s.trial_ends_at,s.created_at,p.name,COALESCE(u.name,''),COALESCE(u.email,''),COALESCE(o.name,'') FROM subscriptions s LEFT JOIN website_pricing_plans p ON p.id=s.plan_id LEFT JOIN users u ON u.id=s.user_id LEFT JOIN organizations o ON o.id=s.organization_id WHERE s.id=$1`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, pgx.ErrNoRows
	}
	return scanSubscription(rows)
}
func parseOptionalTime(s string) (*time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	for _, f := range []string{time.RFC3339, "2006-01-02"} {
		if t, e := time.Parse(f, s); e == nil {
			return &t, nil
		}
	}
	return nil, errors.New("Dates must use YYYY-MM-DD or RFC3339")
}
func (h *PlansBillingHandler) saveSubscription(w http.ResponseWriter, r *http.Request, id int64) {
	creating := id == 0
	var in subscriptionInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid subscription data"})
		return
	}
	if in.PlanID < 1 {
		writeJSON(w, 400, map[string]string{"error": "A valid plan is required"})
		return
	}
	if (in.UserID == nil || *in.UserID < 1) && (in.OrganizationID == nil || *in.OrganizationID < 1) {
		writeJSON(w, 400, map[string]string{"error": "Select a user or organization"})
		return
	}
	if in.Status == "" {
		in.Status = "active"
	}
	if !subscriptionStatuses[in.Status] {
		writeJSON(w, 400, map[string]string{"error": "Invalid subscription status"})
		return
	}
	var price float64
	var currency, period string
	if e := h.DB.QueryRow(r.Context(), `SELECT price,currency,billing_period FROM website_pricing_plans WHERE id=$1 AND archived_at IS NULL`, in.PlanID).Scan(&price, &currency, &period); e != nil {
		writeJSON(w, 400, map[string]string{"error": "Plan not found"})
		return
	}
	for _, ref := range []struct {
		v *int64
		t string
	}{{in.UserID, "users"}, {in.OrganizationID, "organizations"}} {
		if ref.v != nil {
			var ok bool
			if e := h.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM `+ref.t+` WHERE id=$1)`, *ref.v).Scan(&ok); e != nil || !ok {
				writeJSON(w, 400, map[string]string{"error": "Selected user or organization was not found"})
				return
			}
		}
	}
	start, e := parseOptionalTime(in.StartsAt)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": e.Error()})
		return
	}
	end, e := parseOptionalTime(in.EndsAt)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": e.Error()})
		return
	}
	trial, e := parseOptionalTime(in.TrialEndsAt)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": e.Error()})
		return
	}
	if start == nil {
		now := time.Now()
		start = &now
	}
	amount := price
	if in.Amount != nil {
		amount = *in.Amount
	}
	if amount < 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		writeJSON(w, 400, map[string]string{"error": "Amount must be non-negative"})
		return
	}
	if in.Currency != "" {
		currency = strings.ToUpper(in.Currency)
	}
	if len(currency) != 3 || !asciiLetters(currency) {
		writeJSON(w, 400, map[string]string{"error": "Currency must be a three-letter code"})
		return
	}
	if in.BillingPeriod != "" {
		period = in.BillingPeriod
	}
	if !billingPeriods[period] {
		writeJSON(w, 400, map[string]string{"error": "Invalid billing period"})
		return
	}
	var err error
	if id == 0 {
		err = h.DB.QueryRow(r.Context(), `INSERT INTO subscriptions(plan_id,user_id,organization_id,status,started_at,ends_at,trial_ends_at,amount,currency,billing_period,renews_at,is_trial) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$6,($4::varchar='trial' OR $7::timestamptz IS NOT NULL)) RETURNING id`, in.PlanID, in.UserID, in.OrganizationID, in.Status, start, end, trial, amount, currency, period).Scan(&id)
	} else {
		tag, e := h.DB.Exec(r.Context(), `UPDATE subscriptions SET plan_id=$1,user_id=$2,organization_id=$3,status=$4,started_at=$5,ends_at=$6,trial_ends_at=$7,amount=$8,currency=$9,billing_period=$10,renews_at=$6,is_trial=(is_trial OR $4::varchar='trial' OR $7::timestamptz IS NOT NULL),updated_at=NOW() WHERE id=$11`, in.PlanID, in.UserID, in.OrganizationID, in.Status, start, end, trial, amount, currency, period, id)
		err = e
		if e == nil && tag.RowsAffected() == 0 {
			writeJSON(w, 404, map[string]string{"error": "Subscription not found"})
			return
		}
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not save subscription"})
		return
	}
	m, e := h.subscription(r, id)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Subscription saved but could not be loaded"})
		return
	}
	if creating {
		auditAdmin(r, h.DB, "subscription_created", "subscription", id, "Created subscription record")
	} else {
		auditAdmin(r, h.DB, "subscription_updated", "subscription", id, "Updated subscription record")
	}
	writeJSON(w, 200, m)
}
func (h *PlansBillingHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	h.saveSubscription(w, r, 0)
}
func (h *PlansBillingHandler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	id, e := routeID(r)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid subscription id"})
		return
	}
	h.saveSubscription(w, r, id)
}
func (h *PlansBillingHandler) SubscriptionStatus(w http.ResponseWriter, r *http.Request) {
	id, e := routeID(r)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid subscription id"})
		return
	}
	var in struct {
		Status string `json:"status"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || !subscriptionStatuses[in.Status] {
		writeJSON(w, 400, map[string]string{"error": "Invalid subscription status"})
		return
	}
	tag, e := h.DB.Exec(r.Context(), `UPDATE subscriptions SET status=$1,is_trial=(is_trial OR $1::varchar='trial' OR trial_ends_at IS NOT NULL),updated_at=NOW() WHERE id=$2`, in.Status, id)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not update subscription"})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, 404, map[string]string{"error": "Subscription not found"})
		return
	}
	m, e := h.subscription(r, id)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load subscription"})
		return
	}
	auditAdmin(r, h.DB, "subscription_status_changed", "subscription", id, "Subscription status changed to "+in.Status)
	writeJSON(w, 200, m)
}

func routeID(r *http.Request) (int64, error) {
	id, e := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if e != nil || id < 1 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}
