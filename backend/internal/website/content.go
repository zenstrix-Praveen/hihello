package website

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type ContentHandler struct{ DB featureDB }

// GetNamedSection and SaveNamedSection bind a dashboard module to a stable
// section key while keeping all editable content in website_content_sections.
func (h *ContentHandler) GetNamedSection(key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.getSection(w, r, key)
	}
}

func (h *ContentHandler) SaveNamedSection(key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.saveSection(w, r, key)
	}
}

type PricingPlan struct {
	ID            int64           `json:"id"`
	Code          string          `json:"code"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Price         float64         `json:"price"`
	PricingType   string          `json:"pricing_type"`
	Currency      string          `json:"currency"`
	BillingPeriod string          `json:"billing_period"`
	Features      []string        `json:"features"`
	CTAText       string          `json:"cta_text"`
	CTAURL        string          `json:"cta_url"`
	TrialDays     int             `json:"trial_days"`
	Limits        json.RawMessage `json:"limits"`
	IsFeatured    bool            `json:"is_featured"`
	IsEnabled     bool            `json:"is_enabled"`
	SortOrder     int             `json:"sort_order"`
}

func (h *ContentHandler) GetSection(w http.ResponseWriter, r *http.Request) {
	h.getSection(w, r, chi.URLParam(r, "section"))
}

func (h *ContentHandler) getSection(w http.ResponseWriter, r *http.Request, key string) {
	var raw json.RawMessage
	err := h.DB.QueryRow(r.Context(), `SELECT content FROM website_content_sections WHERE section_key=$1`, key).Scan(&raw)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeWebsiteJSON(w, http.StatusOK, map[string]interface{}{})
			return
		}
		writeWebsiteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Could not load section"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}
func (h *ContentHandler) SaveSection(w http.ResponseWriter, r *http.Request) {
	h.saveSection(w, r, chi.URLParam(r, "section"))
}

func (h *ContentHandler) saveSection(w http.ResponseWriter, r *http.Request, key string) {
	if strings.TrimSpace(key) == "" {
		writeWebsiteJSON(w, http.StatusBadRequest, map[string]string{"error": "Section key is required"})
		return
	}
	var data map[string]interface{}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&data) != nil || data == nil || decoder.Decode(new(any)) != io.EOF {
		writeWebsiteJSON(w, 400, map[string]string{"error": "Invalid section content"})
		return
	}
	fields, ok := websiteSectionFields[key]
	if !ok {
		writeWebsiteJSON(w, 400, map[string]string{"error": "Unsupported website section"})
		return
	}
	for name := range data {
		if !fields[name] {
			writeWebsiteJSON(w, 400, map[string]string{"error": "Unsupported field in " + key + " content: " + name})
			return
		}
	}
	if err := validateWebsiteSection(key, data); err != nil {
		writeWebsiteJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	raw, err := json.Marshal(data)
	if err != nil {
		writeWebsiteJSON(w, 400, map[string]string{"error": "Invalid section content"})
		return
	}
	_, err = h.DB.Exec(r.Context(), `INSERT INTO website_content_sections(section_key,content,updated_at) VALUES($1,$2,NOW()) ON CONFLICT(section_key) DO UPDATE SET content=EXCLUDED.content,updated_at=NOW()`, key, raw)
	if err != nil {
		writeWebsiteJSON(w, 500, map[string]string{"error": "Could not save section"})
		return
	}
	writeWebsiteJSON(w, 200, map[string]interface{}{"section": key, "content": data})
}

var websiteSectionFields = map[string]map[string]bool{
	"features":  setFields("enabled", "page_heading", "page_subtitle", "translations"),
	"templates": setFields("enabled", "page_title", "page_subtitle", "hero_text", "cta_text", "cta_url", "translations"),
	"pricing":   setFields("enabled", "page_title", "hero_title", "hero_subtitle", "section_description", "cta_text", "cta_url", "translations"),
	"business":  setFields("enabled", "page_title", "hero_heading", "hero_subheading", "description", "benefits", "cta_heading", "cta_text", "cta_button_label", "cta_url", "contact_sales_email", "image_url", "translations"),
	"contact":   setFields("enabled", "support_email", "sales_email", "phone", "address", "business_hours", "contact_page_heading", "description", "form_heading", "success_message", "translations"),
	"header":    setFields("enabled", "logo_url", "brand_name", "admin_link_label", "platform_menu", "navigation_items", "login_button", "signup_button", "translations"),
	"footer":    setFields("enabled", "logo_url", "brand_name", "company_description", "link_groups", "support_email", "phone", "address", "social_links", "copyright_text", "cta_heading", "cta_description", "cta_primary_text", "cta_primary_url", "cta_secondary_text", "cta_secondary_url", "translations"),
	"seo":       setFields("enabled", "site_name", "title_suffix", "default_title", "meta_description", "meta_keywords", "logo_url", "favicon_url", "public_base_url", "default_og_image_url", "robots_enabled", "google_analytics_id", "default_locale", "translations"),
}

func setFields(keys ...string) map[string]bool {
	out := map[string]bool{}
	for _, k := range keys {
		out[k] = true
	}
	return out
}
func validateWebsiteSection(key string, data map[string]interface{}) error {
	if raw, ok := data["translations"]; ok {
		translations, valid := raw.(map[string]interface{})
		if !valid {
			return errors.New("translations must be an object keyed by language code")
		}
		for language, content := range translations {
			if language != "ar" {
				return errors.New("only Arabic CMS translations are currently supported")
			}
			if _, valid := content.(map[string]interface{}); !valid {
				return errors.New("each CMS language translation must be an object")
			}
		}
	}
	for _, field := range []string{"logo_url", "favicon_url", "image_url", "public_base_url", "default_og_image_url", "support_url", "privacy_policy_url", "terms_url", "cta_url", "cta_primary_url", "cta_secondary_url"} {
		if v, ok := data[field].(string); ok && !safeWebsiteURL(v) {
			return errors.New(field + " must be a safe HTTP(S), mailto or relative URL")
		}
	}
	for _, field := range []string{"support_email", "sales_email", "contact_sales_email"} {
		if v, ok := data[field].(string); ok && v != "" {
			if _, err := mail.ParseAddress(v); err != nil {
				return errors.New(field + " must be a valid email")
			}
		}
	}
	if key == "header" {
		if raw, ok := data["platform_menu"].(map[string]interface{}); ok {
			if label, ok := raw["label"].(string); ok && strings.TrimSpace(label) == "" {
				return errors.New("Platform menu label is required")
			}
			for field := range raw {
				if !setFields("label", "enabled")[field] {
					return errors.New("Unsupported platform menu field: " + field)
				}
			}
		}
		if items, ok := data["navigation_items"].([]interface{}); ok {
			for _, raw := range items {
				item, ok := raw.(map[string]interface{})
				if !ok || strings.TrimSpace(fmt.Sprint(item["label"])) == "" || strings.TrimSpace(fmt.Sprint(item["url"])) == "" || !safeWebsiteURL(fmt.Sprint(item["url"])) {
					return errors.New("Navigation items require a label and safe URL")
				}
				for field := range item {
					if !setFields("label", "url", "sort_order", "enabled", "open_in_new_tab")[field] {
						return errors.New("Unsupported navigation item field: " + field)
					}
				}
			}
		}
		for _, button := range []string{"login_button", "signup_button"} {
			if raw, ok := data[button].(map[string]interface{}); ok {
				if strings.TrimSpace(fmt.Sprint(raw["url"])) == "" || !safeWebsiteURL(fmt.Sprint(raw["url"])) {
					return errors.New(button + " must have a safe URL")
				}
				for field := range raw {
					if !setFields("enabled", "label", "url")[field] {
						return errors.New("Unsupported " + button + " field: " + field)
					}
				}
			}
		}
	}
	if key == "footer" {
		if groups, ok := data["link_groups"].([]interface{}); ok {
			for _, raw := range groups {
				g, ok := raw.(map[string]interface{})
				if !ok {
					return errors.New("Invalid footer link group")
				}
				for field := range g {
					if !setFields("title", "links", "enabled", "sort_order")[field] {
						return errors.New("Unsupported footer group field: " + field)
					}
				}
				if links, ok := g["links"].([]interface{}); ok {
					for _, entry := range links {
						link, ok := entry.(map[string]interface{})
						if !ok || strings.TrimSpace(fmt.Sprint(link["label"])) == "" || strings.TrimSpace(fmt.Sprint(link["url"])) == "" || !safeWebsiteURL(fmt.Sprint(link["url"])) {
							return errors.New("Footer links require a label and safe URL")
						}
						for field := range link {
							if !setFields("label", "url", "sort_order", "enabled")[field] {
								return errors.New("Unsupported footer link field: " + field)
							}
						}
					}
				}
			}
		}
		if socials, ok := data["social_links"].([]interface{}); ok {
			for _, raw := range socials {
				social, ok := raw.(map[string]interface{})
				if !ok || strings.TrimSpace(fmt.Sprint(social["url"])) == "" || !safeWebsiteURL(fmt.Sprint(social["url"])) {
					return errors.New("Social links require a safe URL")
				}
				for field := range social {
					if !setFields("platform", "url", "enabled", "sort_order")[field] {
						return errors.New("Unsupported social link field: " + field)
					}
				}
			}
		}
	}
	return nil
}
func safeWebsiteURL(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") || strings.HasPrefix(value, "#") {
		return true
	}
	u, err := url.Parse(value)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http" || u.Scheme == "mailto" || u.Scheme == "tel") && (u.Scheme == "mailto" || u.Scheme == "tel" || u.Host != "")
}
func (h *ContentHandler) ListPricing(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(), `SELECT id,code,name,description,price,pricing_type,currency,billing_period,features,trial_days,limits,cta_text,cta_url,is_featured,is_enabled,sort_order FROM website_pricing_plans WHERE archived_at IS NULL ORDER BY sort_order,id`)
	if err != nil {
		writeWebsiteJSON(w, 500, map[string]string{"error": "Could not load pricing plans"})
		return
	}
	defer rows.Close()
	items := make([]PricingPlan, 0)
	for rows.Next() {
		var p PricingPlan
		var raw []byte
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.Description, &p.Price, &p.PricingType, &p.Currency, &p.BillingPeriod, &raw, &p.TrialDays, &p.Limits, &p.CTAText, &p.CTAURL, &p.IsFeatured, &p.IsEnabled, &p.SortOrder); err != nil {
			writeWebsiteJSON(w, 500, map[string]string{"error": "Could not read pricing plans"})
			return
		}
		_ = json.Unmarshal(raw, &p.Features)
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		writeWebsiteJSON(w, 500, map[string]string{"error": "Could not read pricing plans"})
		return
	}
	writeWebsiteJSON(w, 200, map[string]interface{}{"data": items})
}

// PublicPricing exposes only enabled plan fields intended for customer selection.
func (h *ContentHandler) PublicPricing(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(), `SELECT id,code,name,description,price,pricing_type,currency,billing_period,trial_days,is_featured,features,limits,cta_text,cta_url,is_most_popular,trial_mode,billing_type,billing_config FROM website_pricing_plans WHERE is_enabled=true AND visibility='public' AND archived_at IS NULL ORDER BY sort_order,id`)
	if err != nil {
		writeWebsiteJSON(w, 500, map[string]string{"error": "Could not load pricing plans"})
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	localizedPlans := map[string]any{}
	translationsFound := false
	if r.URL.Query().Get("lang") == "ar" {
		var contentRaw json.RawMessage
		if err := h.DB.QueryRow(r.Context(), `SELECT content FROM website_content_sections WHERE section_key='pricing'`).Scan(&contentRaw); err == nil {
			var section map[string]any
			if json.Unmarshal(contentRaw, &section) == nil {
				translations, _ := section["translations"].(map[string]any)
				arabic, _ := translations["ar"].(map[string]any)
				localizedPlans, _ = arabic["plans"].(map[string]any)
				translationsFound = len(localizedPlans) > 0
			}
		}
	}
	for rows.Next() {
		var id int64
		var code, name, description, pricingType, currency, period, ctaText, ctaURL string
		var price float64
		var trialDays int
		var featured, popular bool
		var trialMode, billingType string
		var features, limits, billingConfig json.RawMessage
		if err := rows.Scan(&id, &code, &name, &description, &price, &pricingType, &currency, &period, &trialDays, &featured, &features, &limits, &ctaText, &ctaURL, &popular, &trialMode, &billingType, &billingConfig); err != nil {
			writeWebsiteJSON(w, 500, map[string]string{"error": "Could not read pricing plans"})
			return
		}
		var featureValues any = []any{}
		var limitValues any = map[string]any{}
		if len(features) > 0 {
			_ = json.Unmarshal(features, &featureValues)
		}
		if len(limits) > 0 {
			_ = json.Unmarshal(limits, &limitValues)
		}
		item := map[string]any{"id": id, "code": code, "name": name, "description": description, "price": price, "pricing_type": pricingType, "currency": currency, "billing_period": period, "trial_days": trialDays, "trial_mode": trialMode, "billing_type": billingType, "is_featured": featured, "is_most_popular": popular, "cta_text": ctaText, "cta_url": ctaURL, "feature_limits": map[string]any{"features": featureValues, "limits": limitValues}}
		fallback := false
		if r.URL.Query().Get("lang") == "ar" {
			localized, _ := localizedPlans[strconv.FormatInt(id, 10)].(map[string]any)
			if localized != nil {
				for _, field := range []string{"name", "description", "cta_text"} {
					if value, ok := localized[field].(string); ok && strings.TrimSpace(value) != "" {
						item[field] = value
					} else {
						fallback = true
					}
				}
				if features, ok := localized["features"].([]any); ok && len(features) > 0 {
					item["feature_limits"].(map[string]any)["features"] = features
				} else if len(featureValues.([]any)) > 0 {
					fallback = true
				}
			} else {
				fallback = true
			}
			if !translationsFound {
				fallback = true
			}
		}
		item["billing_config"] = billingConfig
		item["_locale_fallback"] = fallback
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeWebsiteJSON(w, 500, map[string]string{"error": "Could not read pricing plans"})
		return
	}
	writeWebsiteJSON(w, 200, map[string]any{"data": items})
}
func (h *ContentHandler) SavePricing(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var p PricingPlan
	if json.NewDecoder(r.Body).Decode(&p) != nil || strings.TrimSpace(p.Name) == "" {
		writeWebsiteJSON(w, 400, map[string]string{"error": "Plan name is required"})
		return
	}
	p.Currency = strings.ToUpper(strings.TrimSpace(p.Currency))
	if len(p.Currency) != 3 || p.Price < 0 {
		writeWebsiteJSON(w, 400, map[string]string{"error": "Enter a three-letter currency and non-negative price"})
		return
	}
	if p.BillingPeriod == "" {
		p.BillingPeriod = "monthly"
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
			writeWebsiteJSON(w, 400, map[string]string{"error": "Free plans must have a zero price and a non-custom billing period"})
			return
		}
	case "fixed":
		if p.Price <= 0 || p.BillingPeriod == "custom" {
			writeWebsiteJSON(w, 400, map[string]string{"error": "Fixed-price plans require a positive price and a purchasable billing period"})
			return
		}
	case "custom":
		p.Price = 0
		p.BillingPeriod = "custom"
	default:
		writeWebsiteJSON(w, 400, map[string]string{"error": "Pricing type must be free, fixed, or custom"})
		return
	}
	if strings.TrimSpace(p.CTAText) == "" {
		p.CTAText = "Get Started"
	}
	if !safeWebsiteURL(p.CTAURL) {
		writeWebsiteJSON(w, 400, map[string]string{"error": "CTA URL must be a safe HTTP(S), mailto or relative URL"})
		return
	}
	if p.Code == "" {
		p.Code = strings.ToLower(strings.TrimSpace(p.Name))
		p.Code = strings.NewReplacer(" ", "-", "_", "-", ".", "-", "/", "-").Replace(p.Code)
		var b strings.Builder
		for _, c := range p.Code {
			if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' {
				b.WriteRune(c)
			}
		}
		p.Code = strings.Trim(b.String(), "-")
		if p.Code == "" {
			p.Code = "plan"
		}
	}
	if p.Features == nil {
		p.Features = []string{}
	}
	if len(p.Limits) == 0 {
		p.Limits = json.RawMessage(`{}`)
	}
	raw, _ := json.Marshal(p.Features)
	var err error
	if id > 0 {
		var tag pgconn.CommandTag
		tag, err = h.DB.Exec(r.Context(), `UPDATE website_pricing_plans SET code=$1,name=$2,description=$3,price=$4,pricing_type=$5,currency=$6,billing_period=$7,features=$8,trial_days=$9,limits=$10,cta_text=$11,cta_url=$12,is_featured=$13,is_enabled=$14,sort_order=$15,updated_at=NOW() WHERE id=$16 AND archived_at IS NULL`, p.Code, p.Name, p.Description, p.Price, p.PricingType, p.Currency, p.BillingPeriod, raw, p.TrialDays, p.Limits, p.CTAText, p.CTAURL, p.IsFeatured, p.IsEnabled, p.SortOrder, id)
		if err == nil && tag.RowsAffected() == 0 {
			writeWebsiteJSON(w, http.StatusNotFound, map[string]string{"error": "Plan not found"})
			return
		}
		p.ID = id
	} else {
		err = h.DB.QueryRow(r.Context(), `INSERT INTO website_pricing_plans(code,name,description,price,pricing_type,currency,billing_period,features,trial_days,limits,cta_text,cta_url,is_featured,is_enabled,sort_order) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING id`, p.Code, p.Name, p.Description, p.Price, p.PricingType, p.Currency, p.BillingPeriod, raw, p.TrialDays, p.Limits, p.CTAText, p.CTAURL, p.IsFeatured, p.IsEnabled, p.SortOrder).Scan(&p.ID)
	}
	if err != nil {
		writeWebsiteJSON(w, 500, map[string]string{"error": "Could not save pricing plan"})
		return
	}
	writeWebsiteJSON(w, 200, p)
}

func (h *ContentHandler) SetPricingStatus(w http.ResponseWriter, r *http.Request) {
	id, e := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if e != nil || id < 1 {
		writeWebsiteJSON(w, 400, map[string]string{"error": "Invalid plan id"})
		return
	}
	var in struct {
		Enabled  *bool `json:"is_enabled"`
		Featured *bool `json:"is_featured"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.Enabled == nil && in.Featured == nil {
		writeWebsiteJSON(w, 400, map[string]string{"error": "A status or featured value is required"})
		return
	}
	tag, e := h.DB.Exec(r.Context(), `UPDATE website_pricing_plans SET is_enabled=COALESCE($1,is_enabled),is_featured=COALESCE($2,is_featured),updated_at=NOW() WHERE id=$3 AND archived_at IS NULL`, in.Enabled, in.Featured, id)
	if e != nil {
		writeWebsiteJSON(w, 500, map[string]string{"error": "Could not update pricing plan"})
		return
	}
	if tag.RowsAffected() == 0 {
		writeWebsiteJSON(w, 404, map[string]string{"error": "Plan not found"})
		return
	}
	writeWebsiteJSON(w, 200, map[string]string{"message": "Pricing plan updated"})
}
func (h *ContentHandler) DeletePricing(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		writeWebsiteJSON(w, 400, map[string]string{"error": "Invalid plan id"})
		return
	}
	tag, err := h.DB.Exec(r.Context(), `UPDATE website_pricing_plans SET is_enabled=false,archived_at=NOW(),updated_at=NOW() WHERE id=$1 AND archived_at IS NULL`, id)
	if err != nil {
		writeWebsiteJSON(w, 500, map[string]string{"error": "Could not delete plan"})
		return
	}
	if tag.RowsAffected() == 0 {
		writeWebsiteJSON(w, 404, map[string]string{"error": "Plan not found"})
		return
	}
	writeWebsiteJSON(w, 200, map[string]string{"message": "Plan deleted"})
}
func writeWebsiteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
