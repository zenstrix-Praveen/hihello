package customer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"digital-card-platform/backend/internal/middleware"
	"github.com/jackc/pgx/v5"
)

// Feature is the stable server-side registry key used by all clients.
type Feature struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Group       string `json:"group"`
}

var FeatureRegistry = []Feature{
	{Code: "digital_cards", Name: "Digital cards", Description: "Maximum digital cards on the account.", Type: "numeric", Group: "Cards"},
	{Code: "qr_codes", Name: "QR codes", Description: "Maximum QR codes on the account.", Type: "numeric", Group: "Cards"},
	{Code: "nfc_tags", Name: "NFC tags", Description: "Maximum NFC tags on the account.", Type: "numeric", Group: "Cards"},
	{Code: "contacts", Name: "Contacts", Description: "Maximum saved contacts.", Type: "numeric", Group: "Account"},
	{Code: "team_members", Name: "Team members", Description: "Maximum team members.", Type: "numeric", Group: "Account"},
	{Code: "storage", Name: "Storage (GB)", Description: "Storage allowance in gigabytes.", Type: "numeric", Group: "Account"},
	{Code: "card_templates", Name: "Card templates", Description: "Maximum available card templates.", Type: "numeric", Group: "Cards"},
	{Code: "analytics", Name: "Analytics", Description: "Access to analytics reporting.", Type: "boolean", Group: "Insights"},
	{Code: "custom_branding", Name: "Custom branding", Description: "Customize the visible brand styling.", Type: "boolean", Group: "Branding"},
	{Code: "custom_domain", Name: "Custom domain", Description: "Use a custom domain.", Type: "boolean", Group: "Account"},
	{Code: "branding_removal", Name: "Branding removal", Description: "Remove platform branding.", Type: "boolean", Group: "Account"},
}

type Entitlement struct {
	Enabled bool `json:"enabled"`
	Limit   int  `json:"limit"`
}

type PlanIdentity struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type SubscriptionState struct {
	Status    string     `json:"status"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type EntitlementResult struct {
	Plan         *PlanIdentity          `json:"plan"`
	Subscription *SubscriptionState     `json:"subscription"`
	Entitlements map[string]Entitlement `json:"entitlements"`
}

// NormalizePlanEntitlements resolves the existing plan features/limits JSON without
// relying on plan display names. Missing registry entries fail closed.
func NormalizePlanEntitlements(featuresJSON, limitsJSON []byte) (map[string]Entitlement, error) {
	var featureCodes []string
	if len(featuresJSON) > 0 && string(featuresJSON) != "null" && json.Unmarshal(featuresJSON, &featureCodes) != nil {
		return nil, errors.New("invalid feature configuration")
	}
	var limits map[string]any
	if len(limitsJSON) > 0 && string(limitsJSON) != "null" && json.Unmarshal(limitsJSON, &limits) != nil {
		return nil, errors.New("invalid limit configuration")
	}
	if limits == nil {
		limits = map[string]any{}
	}
	include := map[string]bool{}
	for _, code := range featureCodes {
		include[strings.ToLower(strings.TrimSpace(code))] = true
	}
	result := make(map[string]Entitlement, len(FeatureRegistry))
	for _, feature := range FeatureRegistry {
		if feature.Type == "numeric" {
			key := feature.Code + "_limit"
			value, found := limits[key]
			if unlimited, ok := limits[feature.Code+"_unlimited"].(bool); ok && unlimited {
				result[feature.Code] = Entitlement{Enabled: true, Limit: -1}
				continue
			}
			if !found {
				result[feature.Code] = Entitlement{Limit: 0}
				continue
			}
			number, ok := value.(float64)
			if !ok || number != float64(int(number)) || number < -1 {
				return nil, errors.New("invalid numeric entitlement")
			}
			result[feature.Code] = Entitlement{Enabled: number != 0, Limit: int(number)}
		} else {
			result[feature.Code] = Entitlement{Enabled: include[feature.Code]}
		}
	}
	return result, nil
}

// CanUse reports whether a feature is enabled and, for numeric features,
// whether another unit can be added. currentUsage is ignored for boolean features.
func (r EntitlementResult) CanUse(code string, currentUsage int) bool {
	e, ok := r.Entitlements[code]
	if !ok || !e.Enabled {
		return false
	}
	return e.Limit == -1 || e.Limit > 0 && currentUsage < e.Limit
}

func (r EntitlementResult) HasFeature(code string) bool {
	e, ok := r.Entitlements[code]
	return ok && e.Enabled
}
func (r EntitlementResult) GetLimit(code string) int {
	if e, ok := r.Entitlements[code]; ok {
		return e.Limit
	}
	return 0
}
func (r EntitlementResult) IsUnlimited(code string) bool { return r.GetLimit(code) == -1 }
func (r EntitlementResult) Remaining(code string, usage int) int {
	limit := r.GetLimit(code)
	if limit == -1 {
		return -1
	}
	if limit <= usage {
		return 0
	}
	return limit - usage
}

// ResolveEntitlements finds the latest subscription record for a user and
// returns its valid plan configuration, or the enabled Free-type fallback.
func (h *Handler) ResolveEntitlements(ctx context.Context, userID int64) (EntitlementResult, error) {
	var subID int64
	var status string
	var starts time.Time
	var ends, trialEnds *time.Time
	var planID *int64
	err := h.DB.QueryRow(ctx, `SELECT id,status,started_at,ends_at,trial_ends_at,plan_id FROM subscriptions WHERE user_id=$1 ORDER BY started_at DESC,id DESC LIMIT 1`, userID).Scan(&subID, &status, &starts, &ends, &trialEnds, &planID)
	var identity *PlanIdentity
	var state *SubscriptionState
	var featureJSON, limitsJSON []byte
	now := time.Now()
	valid := err == nil && planID != nil && (status == "active" || status == "trial" || status == "past_due") && !starts.After(now) && (ends == nil || ends.After(now)) && (status != "trial" || trialEnds == nil || trialEnds.After(now))
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	if err != nil {
		return EntitlementResult{}, err
	}
	if valid {
		var id int64
		var code, name string
		err = h.DB.QueryRow(ctx, `SELECT id,code,name,features,limits FROM website_pricing_plans WHERE id=$1`, *planID).Scan(&id, &code, &name, &featureJSON, &limitsJSON)
		if err == nil {
			identity = &PlanIdentity{id, code, name}
			state = &SubscriptionState{status, ends}
		}
		if errors.Is(err, pgx.ErrNoRows) {
			valid = false
		} else if err != nil {
			return EntitlementResult{}, err
		}
	}
	if !valid {
		var fallbackCode, fallbackName string
		err = h.DB.QueryRow(ctx, `SELECT id,code,name,features,limits FROM website_pricing_plans WHERE pricing_type='free' AND is_enabled=true ORDER BY sort_order,id LIMIT 1`).Scan(&subID, &fallbackCode, &fallbackName, &featureJSON, &limitsJSON)
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
			featureJSON = []byte(`[]`)
			limitsJSON = []byte(`{}`)
		} else if err != nil {
			return EntitlementResult{}, err
		} else {
			identity = &PlanIdentity{subID, fallbackCode, fallbackName}
		}
	}
	entitlements, err := NormalizePlanEntitlements(featureJSON, limitsJSON)
	if err != nil {
		return EntitlementResult{}, err
	}
	return EntitlementResult{Plan: identity, Subscription: state, Entitlements: entitlements}, nil
}

func (h *Handler) Entitlements(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok || userID < 1 {
		respond(w, 401, map[string]string{"error": "A valid user session is required"})
		return
	}
	result, err := h.ResolveEntitlements(r.Context(), userID)
	if err != nil {
		respond(w, 500, map[string]string{"error": "Could not resolve entitlements"})
		return
	}
	respond(w, 200, map[string]any{"data": result})
}
