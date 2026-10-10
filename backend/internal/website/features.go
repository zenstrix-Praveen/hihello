package website

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Implemented by pgxpool.Pool; keeps contract tests independent of a live DB.
type featureDB interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Begin(context.Context) (pgx.Tx, error)
}

type FeatureHandler struct{ DB featureDB }

func featureError(w http.ResponseWriter, status int, message string) {
	// Keep error for both existing API clients, which read it before message.
	writeWebsiteJSON(w, status, map[string]any{
		"success": false, "message": message, "error": message,
	})
}

type featureListResponse struct {
	featureSuccess
	Items []Feature `json:"items"`
}

type featureSuccess struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// Preserve top-level record fields for existing create/update consumers.
type featureRecordResponse struct {
	featureSuccess
	Feature
}

type Feature struct {
	ID                  int64  `json:"id"`
	Title               string `json:"title"`
	Description         string `json:"description"`
	DetailedDescription string `json:"detailed_description"`
	IconKey             string `json:"icon_key"`
	IllustrationURL     string `json:"illustration_url"`
	SortOrder           int    `json:"sort_order"`
	IsEnabled           bool   `json:"is_enabled"`
	CTALabel            string `json:"cta_label"`
	CTAURL              string `json:"cta_url"`
}

const featureColumns = `id,title,description,detailed_description,icon_key,illustration_url,sort_order,is_enabled,cta_label,cta_url`

func (h *FeatureHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(), `SELECT `+featureColumns+` FROM website_features ORDER BY sort_order,id`)
	if err != nil {
		log.Printf("FeatureHandler.List query failed: %v", err)
		featureError(w, 500, "Could not load features")
		return
	}
	defer rows.Close()
	items := make([]Feature, 0)
	for rows.Next() {
		var f Feature
		if err := rows.Scan(&f.ID, &f.Title, &f.Description, &f.DetailedDescription, &f.IconKey, &f.IllustrationURL, &f.SortOrder, &f.IsEnabled, &f.CTALabel, &f.CTAURL); err != nil {
			log.Printf("FeatureHandler.List row scan failed: %v", err)
			featureError(w, 500, "Could not read features")
			return
		}
		items = append(items, f)
	}
	if rows.Err() != nil {
		log.Printf("FeatureHandler.List rows failed: %v", rows.Err())
		featureError(w, 500, "Could not read features")
		return
	}
	writeWebsiteJSON(w, 200, featureListResponse{
		featureSuccess: featureSuccess{true, "Features retrieved successfully", items},
		Items:          items,
	})
}

func (h *FeatureHandler) PublicList(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(), `SELECT `+featureColumns+` FROM website_features WHERE is_enabled=true ORDER BY sort_order,id`)
	if err != nil {
		featureError(w, 500, "Could not load features")
		return
	}
	defer rows.Close()
	items := make([]Feature, 0)
	for rows.Next() {
		var f Feature
		if err := rows.Scan(&f.ID, &f.Title, &f.Description, &f.DetailedDescription, &f.IconKey, &f.IllustrationURL, &f.SortOrder, &f.IsEnabled, &f.CTALabel, &f.CTAURL); err != nil {
			featureError(w, 500, "Could not read features")
			return
		}
		items = append(items, f)
	}
	if rows.Err() != nil {
		featureError(w, 500, "Could not read features")
		return
	}
	var page map[string]any
	var raw []byte
	if err := h.DB.QueryRow(r.Context(), `SELECT content FROM website_content_sections WHERE section_key='features'`).Scan(&raw); err == nil {
		_ = json.Unmarshal(raw, &page)
	}
	if page == nil {
		page = map[string]any{}
	}
	if enabled, exists := page["enabled"].(bool); exists && !enabled {
		featureError(w, http.StatusNotFound, "Content is not published")
		return
	}
	fallback := false
	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("lang")), "ar") {
		translations, _ := page["translations"].(map[string]interface{})
		localized, _ := translations["ar"].(map[string]interface{})
		for _, field := range []string{"page_heading", "page_subtitle"} {
			if value, ok := localized[field].(string); ok && strings.TrimSpace(value) != "" { page[field] = value } else { fallback = true }
		}
		localizedItems, _ := localized["feature_items"].(map[string]interface{})
		for index := range items {
			item := &items[index]
			entry, _ := localizedItems[strconv.FormatInt(item.ID,10)].(map[string]interface{})
			for _, field := range []string{"title","description","detailed_description","cta_label"} {
				value, ok := entry[field].(string)
				if ok && strings.TrimSpace(value) != "" {
					switch field { case "title": item.Title=value; case "description": item.Description=value; case "detailed_description": item.DetailedDescription=value; case "cta_label": item.CTALabel=value }
				} else { fallback = true }
			}
		}
	}
	delete(page, "translations")
	page["items"] = items
	page["_locale_fallback"] = fallback
	page["success"] = true
	page["message"] = "Features retrieved successfully"
	page["data"] = items
	writeWebsiteJSON(w, 200, page)
}

func decodeFeature(r *http.Request, in *Feature) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(in); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return err
	}
	return nil
}
func (h *FeatureHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in Feature
	if decodeFeature(r, &in) != nil || strings.TrimSpace(in.Title) == "" {
		featureError(w, http.StatusUnprocessableEntity, "Feature title is required")
		return
	}
	if !safeWebsiteURL(in.IllustrationURL) || !safeWebsiteURL(in.CTAURL) {
		featureError(w, http.StatusUnprocessableEntity, "Feature URLs must be safe HTTP(S) or relative URLs")
		return
	}
	err := h.DB.QueryRow(r.Context(), `INSERT INTO website_features(title,description,detailed_description,icon_key,illustration_url,sort_order,is_enabled,cta_label,cta_url) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`, in.Title, in.Description, in.DetailedDescription, in.IconKey, in.IllustrationURL, in.SortOrder, in.IsEnabled, in.CTALabel, in.CTAURL).Scan(&in.ID)
	if err != nil {
		featureError(w, 500, "Could not create feature")
		return
	}
	writeWebsiteJSON(w, 201, featureRecordResponse{
		featureSuccess: featureSuccess{true, "Feature created successfully", in},
		Feature:        in,
	})
}

func (h *FeatureHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		featureError(w, 400, "Invalid feature id")
		return
	}
	var in Feature
	if decodeFeature(r, &in) != nil || strings.TrimSpace(in.Title) == "" {
		featureError(w, http.StatusUnprocessableEntity, "Feature title is required")
		return
	}
	if !safeWebsiteURL(in.IllustrationURL) || !safeWebsiteURL(in.CTAURL) {
		featureError(w, http.StatusUnprocessableEntity, "Feature URLs must be safe HTTP(S) or relative URLs")
		return
	}
	tag, err := h.DB.Exec(r.Context(), `UPDATE website_features SET title=$1,description=$2,detailed_description=$3,icon_key=$4,illustration_url=$5,sort_order=$6,is_enabled=$7,cta_label=$8,cta_url=$9,updated_at=NOW() WHERE id=$10`, in.Title, in.Description, in.DetailedDescription, in.IconKey, in.IllustrationURL, in.SortOrder, in.IsEnabled, in.CTALabel, in.CTAURL, id)
	if err != nil {
		featureError(w, 500, "Could not update feature")
		return
	}
	if tag.RowsAffected() == 0 {
		featureError(w, 404, "Feature not found")
		return
	}
	in.ID = id
	writeWebsiteJSON(w, 200, featureRecordResponse{
		featureSuccess: featureSuccess{true, "Feature updated successfully", in},
		Feature:        in,
	})
}

func (h *FeatureHandler) SetOrder(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.IDs) > 500 {
		featureError(w, http.StatusUnprocessableEntity, "Invalid feature order")
		return
	}
	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		featureError(w, 500, "Could not reorder features")
		return
	}
	defer tx.Rollback(r.Context())
	for i, id := range in.IDs {
		if id < 1 {
			featureError(w, 400, "Invalid feature id")
			return
		}
		tag, e := tx.Exec(r.Context(), `UPDATE website_features SET sort_order=$1,updated_at=NOW() WHERE id=$2`, i, id)
		if e != nil || tag.RowsAffected() == 0 {
			featureError(w, 400, "Feature order contains an unknown item")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		featureError(w, 500, "Could not save feature order")
		return
	}
	writeWebsiteJSON(w, 200, featureSuccess{true, "Feature order saved", nil})
}

func (h *FeatureHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		featureError(w, 400, "Invalid feature id")
		return
	}
	tag, err := h.DB.Exec(r.Context(), `DELETE FROM website_features WHERE id=$1`, id)
	if err != nil {
		featureError(w, 500, "Could not delete feature")
		return
	}
	if tag.RowsAffected() == 0 {
		featureError(w, 404, "Feature not found")
		return
	}
	writeWebsiteJSON(w, 200, featureSuccess{true, "Feature deleted", nil})
}
