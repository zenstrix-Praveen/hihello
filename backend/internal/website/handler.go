package website

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	DB *pgxpool.Pool
}

type HomeContent struct {
	HeroTitle           string                       `json:"hero_title"`
	HeroSubtitle        string                       `json:"hero_subtitle"`
	PrimaryButtonText   string                       `json:"primary_button_text"`
	PrimaryButtonURL    string                       `json:"primary_button_url"`
	SecondaryButtonText string                       `json:"secondary_button_text"`
	SecondaryButtonURL  string                       `json:"secondary_button_url"`
	HeroImageURL        string                       `json:"hero_image_url"`
	Translations        map[string]map[string]string `json:"translations,omitempty"`
	LocaleFallback      bool                         `json:"_locale_fallback,omitempty"`
}

func (h *Handler) GetHome(w http.ResponseWriter, r *http.Request) {
	var item HomeContent

	err := h.DB.QueryRow(
		r.Context(),
		`
        SELECT
            hero_title,
            hero_subtitle,
            primary_button_text,
            primary_button_url,
            secondary_button_text,
            secondary_button_url,
            hero_image_url
        FROM website_home
        WHERE id = 1
        `,
	).Scan(
		&item.HeroTitle,
		&item.HeroSubtitle,
		&item.PrimaryButtonText,
		&item.PrimaryButtonURL,
		&item.SecondaryButtonText,
		&item.SecondaryButtonURL,
		&item.HeroImageURL,
	)

	if err != nil {
		http.Error(w, `{"error":"Could not load home page"}`, http.StatusInternalServerError)
		return
	}
	var translations map[string]map[string]string
	var raw []byte
	if e := h.DB.QueryRow(r.Context(), `SELECT content FROM website_content_sections WHERE section_key='home'`).Scan(&raw); e == nil {
		var saved struct {
			Translations map[string]map[string]string `json:"translations"`
		}
		if json.Unmarshal(raw, &saved) == nil {
			translations = saved.Translations
		}
	}
	item.Translations = translations
	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("lang")), "ar") {
		localized := translations["ar"]
		item.LocaleFallback = len(localized) == 0
		if localized["hero_title"] != "" {
			item.HeroTitle = localized["hero_title"]
		} else {
			item.LocaleFallback = true
		}
		if localized["hero_subtitle"] != "" {
			item.HeroSubtitle = localized["hero_subtitle"]
		} else {
			item.LocaleFallback = true
		}
		if localized["primary_button_text"] != "" {
			item.PrimaryButtonText = localized["primary_button_text"]
		} else {
			item.LocaleFallback = true
		}
		if localized["secondary_button_text"] != "" {
			item.SecondaryButtonText = localized["secondary_button_text"]
		} else {
			item.LocaleFallback = true
		}
		item.Translations = nil
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func (h *Handler) UpdateHome(w http.ResponseWriter, r *http.Request) {
	var input HomeContent
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, `{"error":"Invalid request"}`, http.StatusBadRequest)
		return
	}
	if decoder.Decode(new(any)) != io.EOF {
		writeWebsiteJSON(w, http.StatusBadRequest, map[string]string{"error": "Only one JSON object is allowed"})
		return
	}
	if strings.TrimSpace(input.HeroTitle) == "" {
		writeWebsiteJSON(w, http.StatusBadRequest, map[string]string{"error": "Hero title is required"})
		return
	}
	for language, fields := range input.Translations {
		if language != "ar" {
			writeWebsiteJSON(w, 400, map[string]string{"error": "Only Arabic CMS translations are currently supported"})
			return
		}
		for field := range fields {
			if !setFields("hero_title", "hero_subtitle", "primary_button_text", "secondary_button_text")[field] {
				writeWebsiteJSON(w, 400, map[string]string{"error": "Unsupported localized Home field"})
				return
			}
		}
	}

	if !safeWebsiteURL(input.PrimaryButtonURL) || !safeWebsiteURL(input.SecondaryButtonURL) || !safeWebsiteURL(input.HeroImageURL) {
		writeWebsiteJSON(w, http.StatusBadRequest, map[string]string{"error": "Home page URLs must be safe HTTP(S) or relative URLs"})
		return
	}
	tag, err := h.DB.Exec(
		r.Context(),
		`
        UPDATE website_home
        SET
            hero_title = $1,
            hero_subtitle = $2,
            primary_button_text = $3,
            primary_button_url = $4,
            secondary_button_text = $5,
            secondary_button_url = $6,
            hero_image_url = $7,
            updated_at = NOW()
        WHERE id = 1
        `,
		input.HeroTitle,
		input.HeroSubtitle,
		input.PrimaryButtonText,
		input.PrimaryButtonURL,
		input.SecondaryButtonText,
		input.SecondaryButtonURL,
		input.HeroImageURL,
	)

	if err != nil {
		writeWebsiteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Could not save home page"})
		return
	}
	if tag.RowsAffected() == 0 {
		writeWebsiteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Home page settings row is missing"})
		return
	}
	if input.Translations != nil {
		var existing map[string]interface{}
		var savedRaw []byte
		if e := h.DB.QueryRow(r.Context(), `SELECT content FROM website_content_sections WHERE section_key='home'`).Scan(&savedRaw); e == nil {
			_ = json.Unmarshal(savedRaw, &existing)
		}
		if existing == nil {
			existing = map[string]interface{}{}
		}
		existing["translations"] = input.Translations
		raw, _ := json.Marshal(existing)
		if _, err := h.DB.Exec(r.Context(), `INSERT INTO website_content_sections(section_key,content,updated_at) VALUES('home',$1,NOW()) ON CONFLICT(section_key) DO UPDATE SET content=EXCLUDED.content,updated_at=NOW()`, raw); err != nil {
			writeWebsiteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Could not save Home translations"})
			return
		}
	} else {
		var savedRaw []byte
		if e := h.DB.QueryRow(r.Context(), `SELECT content FROM website_content_sections WHERE section_key='home'`).Scan(&savedRaw); e == nil {
			var existing map[string]interface{}
			if json.Unmarshal(savedRaw, &existing) == nil {
				if _, ok := existing["translations"]; ok {
					input.Translations = map[string]map[string]string{}
					_ = json.Unmarshal(rawTranslations(existing["translations"]), &input.Translations)
				}
			}
		}
	}
	writeWebsiteJSON(w, http.StatusOK, input)
}

func rawTranslations(value interface{}) []byte { raw, _ := json.Marshal(value); return raw }

// ResetHome clears custom hero content without removing the singleton row.
// Empty database defaults activate the public Home page's existing fallbacks.
func (h *Handler) ResetHome(w http.ResponseWriter, r *http.Request) {
	tag, err := h.DB.Exec(r.Context(), `
  UPDATE website_home SET
   hero_title=DEFAULT, hero_subtitle=DEFAULT,
   primary_button_text=DEFAULT, primary_button_url=DEFAULT,
   secondary_button_text=DEFAULT, secondary_button_url=DEFAULT,
   hero_image_url=DEFAULT, updated_at=NOW()
  WHERE id=1`)
	if err != nil {
		writeWebsiteJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "message": "Could not reset Home Page content.", "error": "Could not reset Home Page content."})
		return
	}
	if tag.RowsAffected() == 0 {
		writeWebsiteJSON(w, http.StatusNotFound, map[string]any{"success": false, "message": "Home Page settings not found.", "error": "Home Page settings not found."})
		return
	}
	if _, err := h.DB.Exec(r.Context(), `DELETE FROM website_content_sections WHERE section_key='home'`); err != nil {
		writeWebsiteJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "message": "Could not reset Home Page translations.", "error": "Could not reset Home Page translations."})
		return
	}
	writeWebsiteJSON(w, http.StatusOK, map[string]any{"success": true, "message": "Home Page content reset successfully.", "data": HomeContent{}})
}
