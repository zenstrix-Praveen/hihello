package website

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PublicTemplateHandler struct {
	DB *pgxpool.Pool
}

type PublicTemplate struct {
	ID              int64           `json:"id"`
	Name            string          `json:"name"`
	Slug            string          `json:"slug"`
	Description     string          `json:"description"`
	PreviewImageURL string          `json:"preview_image_url"`
	Category        string          `json:"category"`
	DesignSettings  json.RawMessage `json:"design_settings"`
	LocaleFallback  bool            `json:"_locale_fallback,omitempty"`
}

func (h *PublicTemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(), `
        SELECT
            id,
            name,
            slug,
            COALESCE(description, ''),
            COALESCE(preview_image_url, ''),
            COALESCE(category, '')
            ,design_settings
        FROM website_templates
        WHERE is_enabled = true
        ORDER BY sort_order ASC, id ASC
    `)
	if err != nil {
		log.Printf("PublicTemplates query failed: %v", err)
		writePublicTemplateJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not load templates",
		})
		return
	}
	defer rows.Close()

	items := []PublicTemplate{}
	localizedTemplates := map[string]any{}
	translationsFound := false
	if r.URL.Query().Get("lang") == "ar" {
		var raw json.RawMessage
		if err := h.DB.QueryRow(r.Context(), `SELECT content FROM website_content_sections WHERE section_key='templates'`).Scan(&raw); err == nil {
			var section map[string]any
			if json.Unmarshal(raw, &section) == nil {
				translations, _ := section["translations"].(map[string]any)
				arabic, _ := translations["ar"].(map[string]any)
				localizedTemplates, _ = arabic["templates"].(map[string]any)
				translationsFound = len(localizedTemplates) > 0
			}
		}
	}

	for rows.Next() {
		var item PublicTemplate

		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Slug,
			&item.Description,
			&item.PreviewImageURL,
			&item.Category,
			&item.DesignSettings,
		); err != nil {
			log.Printf("PublicTemplates row scan failed: %v", err)
			writePublicTemplateJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Could not read templates",
			})
			return
		}
		if r.URL.Query().Get("lang") == "ar" {
			localized, _ := localizedTemplates[fmt.Sprint(item.ID)].(map[string]any)
			if localized == nil || !translationsFound {
				item.LocaleFallback = true
			} else {
				if value, ok := localized["name"].(string); ok && value != "" {
					item.Name = value
				} else {
					item.LocaleFallback = true
				}
				if value, ok := localized["description"].(string); ok && value != "" {
					item.Description = value
				} else if item.Description != "" {
					item.LocaleFallback = true
				}
				if value, ok := localized["category"].(string); ok && value != "" {
					item.Category = value
				} else if item.Category != "" {
					item.LocaleFallback = true
				}
			}
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		log.Printf("PublicTemplates rows failed: %v", err)
		writePublicTemplateJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not read templates",
		})
		return
	}

	writePublicTemplateJSON(w, http.StatusOK, items)
}

func writePublicTemplateJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
