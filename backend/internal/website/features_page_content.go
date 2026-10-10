package website

import (
	"encoding/json"
	"net/http"
)

// These are the existing public /features fallbacks. Cards are stored separately.
func defaultFeaturesPageContent() map[string]any {
	return map[string]any{
		"page_heading":  "Everything Your Digital Card Needs",
		"page_subtitle": "Everything you need to create, share and grow your professional digital identity.",
		"enabled":       true,
	}
}

func (h *ContentHandler) ResetFeaturesPageContent(w http.ResponseWriter, r *http.Request) {
	defaults := defaultFeaturesPageContent()
	raw, _ := json.Marshal(defaults)
	_, err := h.DB.Exec(r.Context(), `INSERT INTO website_content_sections(section_key,content,updated_at) VALUES($1,$2,NOW()) ON CONFLICT(section_key) DO UPDATE SET content=EXCLUDED.content,updated_at=NOW()`, "features", raw)
	if err != nil {
		featureError(w, http.StatusInternalServerError, "Could not reset Features page content.")
		return
	}
	writeWebsiteJSON(w, http.StatusOK, featureSuccess{true, "Features page content reset to defaults.", defaults})
}

func (h *ContentHandler) DeleteFeaturesPageContent(w http.ResponseWriter, r *http.Request) {
	_, err := h.DB.Exec(r.Context(), `DELETE FROM website_content_sections WHERE section_key=$1`, "features")
	if err != nil {
		featureError(w, http.StatusInternalServerError, "Could not delete Features page content.")
		return
	}
	writeWebsiteJSON(w, http.StatusOK, featureSuccess{true, "Custom Features page content deleted.", nil})
}
