package website

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PublicContentHandler struct{ DB *pgxpool.Pool }

func (h *PublicContentHandler) Section(key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var raw []byte
		e := h.DB.QueryRow(r.Context(), `SELECT content FROM website_content_sections WHERE section_key=$1`, key).Scan(&raw)
		if e == pgx.ErrNoRows {
			writeWebsiteJSON(w, 200, map[string]interface{}{})
			return
		}
		if e != nil {
			writeWebsiteJSON(w, 500, map[string]string{"error": "Could not load website content"})
			return
		}
		var data map[string]interface{}
		if json.Unmarshal(raw, &data) != nil {
			writeWebsiteJSON(w, 500, map[string]string{"error": "Website content is invalid"})
			return
		}
		if allowed, ok := websiteSectionFields[key]; ok {
			safe := map[string]interface{}{}
			for field, value := range data {
				if allowed[field] && field != "translations" {
					safe[field] = value
				}
			}
			translations, _ := data["translations"].(map[string]interface{})
			language := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("lang")))
			fallback := false
			if language == "ar" {
				if localized, ok := translations["ar"].(map[string]interface{}); ok && len(localized) > 0 {
					fallback = hasUntranslatedWebsiteText(safe, localized, "")
					for field, value := range localized {
						if allowed[field] && field != "translations" {
							if original, exists := safe[field]; exists {
								safe[field] = overlayWebsiteContent(original, value)
							} else {
								safe[field] = value
							}
						}
					}
				} else {
					fallback = true
				}
			} else if language != "" && language != "en" {
				fallback = true
			}
			safe["_locale_fallback"] = fallback
			data = safe
			raw, _ = json.Marshal(data)
		}
		if enabled, ok := data["enabled"].(bool); ok && !enabled {
			writeWebsiteJSON(w, 404, map[string]string{"error": "Content is not published"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(raw)
	}
}

func hasUntranslatedWebsiteText(english, localized interface{}, field string) bool {
	switch base := english.(type) {
	case map[string]interface{}:
		patch, _ := localized.(map[string]interface{})
		for key, value := range base {
			if isNonTranslatableWebsiteField(key) {
				continue
			}
			if hasUntranslatedWebsiteText(value, patch[key], key) {
				return true
			}
		}
	case []interface{}:
		patch, _ := localized.([]interface{})
		for index, value := range base {
			var translated interface{}
			if index < len(patch) {
				translated = patch[index]
			}
			if hasUntranslatedWebsiteText(value, translated, field) {
				return true
			}
		}
	case string:
		if strings.TrimSpace(base) == "" {
			return false
		}
		translated, ok := localized.(string)
		return !ok || strings.TrimSpace(translated) == ""
	}
	return false
}

func isNonTranslatableWebsiteField(field string) bool {
	key := strings.ToLower(field)
	for _, part := range []string{"url", "email", "phone", "slug", "code", "_id", "id", "image", "logo", "favicon", "enabled", "order", "sort", "locale", "brand_name"} {
		if key == part || strings.HasSuffix(key, "_"+part) {
			return true
		}
	}
	return false
}

// overlayWebsiteContent replaces only supplied localized text and keeps English
// siblings intact, including child CMS items omitted from an Arabic translation.
func overlayWebsiteContent(english, localized interface{}) interface{} {
	switch base := english.(type) {
	case map[string]interface{}:
		patch, ok := localized.(map[string]interface{})
		if !ok {
			return localized
		}
		out := make(map[string]interface{}, len(base))
		for key, value := range base {
			out[key] = value
		}
		for key, value := range patch {
			if old, exists := out[key]; exists {
				out[key] = overlayWebsiteContent(old, value)
			} else {
				out[key] = value
			}
		}
		return out
	case []interface{}:
		patch, ok := localized.([]interface{})
		if !ok {
			return localized
		}
		out := append([]interface{}(nil), base...)
		for index, value := range patch {
			if index < len(out) {
				out[index] = overlayWebsiteContent(out[index], value)
			} else {
				out = append(out, value)
			}
		}
		return out
	default:
		if localized == nil || fmt.Sprint(localized) == "" {
			return english
		}
		return localized
	}
}

type enquiryInput struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Subject string `json:"subject"`
	Message string `json:"message"`
}

var enquiryStatuses = map[string]bool{"new": true, "in_progress": true, "resolved": true, "spam": true}

func (h *PublicContentHandler) SubmitContact(w http.ResponseWriter, r *http.Request) {
	var in enquiryInput
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	if dec.Decode(&in) != nil || dec.Decode(new(any)) != io.EOF {
		writeWebsiteJSON(w, 400, map[string]string{"error": "Invalid contact request"})
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.TrimSpace(in.Email)
	in.Phone = strings.TrimSpace(in.Phone)
	in.Subject = strings.TrimSpace(in.Subject)
	in.Message = strings.TrimSpace(in.Message)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 150 || in.Subject == "" || utf8.RuneCountInString(in.Subject) > 200 || in.Message == "" || len(in.Message) > 20000 {
		writeWebsiteJSON(w, 400, map[string]string{"error": "Name, subject and message are required"})
		return
	}
	parsedEmail, e := mail.ParseAddress(in.Email)
	if e != nil || strings.TrimSpace(parsedEmail.Address) != in.Email || len(parsedEmail.Address) > 255 {
		writeWebsiteJSON(w, 400, map[string]string{"error": "Enter a valid email address"})
		return
	}
	in.Email = strings.ToLower(parsedEmail.Address)
	if len(in.Phone) > 50 {
		writeWebsiteJSON(w, 400, map[string]string{"error": "Phone is too long"})
		return
	}
	_, e = h.DB.Exec(r.Context(), `INSERT INTO contacts(name,email,phone,subject,message,source,status,created_at) VALUES($1,$2,NULLIF($3,''),$4,$5,'website','new',NOW())`, in.Name, in.Email, in.Phone, in.Subject, in.Message)
	if e != nil {
		writeWebsiteJSON(w, 500, map[string]string{"error": "Could not submit contact request"})
		return
	}
	writeWebsiteJSON(w, 201, map[string]string{"message": "Your message has been received"})
}

func (h *PublicContentHandler) ListEnquiries(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status != "" && !enquiryStatuses[status] {
		writeWebsiteJSON(w, 400, map[string]string{"error": "Invalid enquiry status"})
		return
	}
	search := "%" + strings.TrimSpace(r.URL.Query().Get("search")) + "%"
	rows, e := h.DB.Query(r.Context(), `SELECT id,name,email,COALESCE(phone,''),subject,message,status,notes,created_at FROM contacts WHERE source='website' AND ($1='' OR status=$1) AND ($2='%%' OR name ILIKE $2 OR email ILIKE $2 OR subject ILIKE $2) ORDER BY created_at DESC LIMIT 500`, status, search)
	if e != nil {
		writeWebsiteJSON(w, 500, map[string]string{"error": "Could not load contact enquiries"})
		return
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var id int64
		var name, email, phone, subject, message, status, notes string
		var created time.Time
		if rows.Scan(&id, &name, &email, &phone, &subject, &message, &status, &notes, &created) != nil {
			writeWebsiteJSON(w, 500, map[string]string{"error": "Could not read contact enquiries"})
			return
		}
		out = append(out, map[string]interface{}{"id": id, "name": name, "email": email, "phone": phone, "subject": subject, "message": message, "status": status, "internal_note": notes, "submitted_at": created})
	}
	writeWebsiteJSON(w, 200, map[string]interface{}{"data": out})
}
func (h *PublicContentHandler) GetEnquiry(w http.ResponseWriter, r *http.Request) {
	id, e := enquiryID(r)
	if e != nil {
		writeWebsiteJSON(w, 400, map[string]string{"error": "Invalid enquiry id"})
		return
	}
	var name, email, phone, subject, message, status, notes string
	var created time.Time
	e = h.DB.QueryRow(r.Context(), `SELECT name,email,COALESCE(phone,''),subject,message,status,notes,created_at FROM contacts WHERE id=$1 AND source='website'`, id).Scan(&name, &email, &phone, &subject, &message, &status, &notes, &created)
	if e == pgx.ErrNoRows {
		writeWebsiteJSON(w, 404, map[string]string{"error": "Enquiry not found"})
		return
	}
	if e != nil {
		writeWebsiteJSON(w, 500, map[string]string{"error": "Could not load enquiry"})
		return
	}
	writeWebsiteJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"id": id, "name": name, "email": email, "phone": phone, "subject": subject, "message": message, "status": status, "internal_note": notes, "submitted_at": created}})
}
func (h *PublicContentHandler) SetEnquiryStatus(w http.ResponseWriter, r *http.Request) {
	id, e := enquiryID(r)
	if e != nil {
		writeWebsiteJSON(w, 400, map[string]string{"error": "Invalid enquiry id"})
		return
	}
	var in struct {
		Status string `json:"status"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || !enquiryStatuses[in.Status] {
		writeWebsiteJSON(w, 400, map[string]string{"error": "Invalid enquiry status"})
		return
	}
	tag, e := h.DB.Exec(r.Context(), `UPDATE contacts SET status=$1 WHERE id=$2 AND source='website'`, in.Status, id)
	if e != nil {
		writeWebsiteJSON(w, 500, map[string]string{"error": "Could not update enquiry"})
		return
	}
	if tag.RowsAffected() == 0 {
		writeWebsiteJSON(w, 404, map[string]string{"error": "Enquiry not found"})
		return
	}
	writeWebsiteJSON(w, 200, map[string]string{"message": "Enquiry status updated"})
}
func (h *PublicContentHandler) SetEnquiryNote(w http.ResponseWriter, r *http.Request) {
	id, e := enquiryID(r)
	if e != nil {
		writeWebsiteJSON(w, 400, map[string]string{"error": "Invalid enquiry id"})
		return
	}
	var in struct {
		Note string `json:"internal_note"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.Note) > 10000 {
		writeWebsiteJSON(w, 400, map[string]string{"error": "Invalid internal note"})
		return
	}
	tag, e := h.DB.Exec(r.Context(), `UPDATE contacts SET notes=$1 WHERE id=$2 AND source='website'`, strings.TrimSpace(in.Note), id)
	if e != nil {
		writeWebsiteJSON(w, 500, map[string]string{"error": "Could not save internal note"})
		return
	}
	if tag.RowsAffected() == 0 {
		writeWebsiteJSON(w, 404, map[string]string{"error": "Enquiry not found"})
		return
	}
	writeWebsiteJSON(w, 200, map[string]string{"message": "Internal note saved"})
}
func (h *PublicContentHandler) DeleteEnquiry(w http.ResponseWriter, r *http.Request) {
	id, e := enquiryID(r)
	if e != nil {
		writeWebsiteJSON(w, 400, map[string]string{"error": "Invalid enquiry id"})
		return
	}
	tag, e := h.DB.Exec(r.Context(), `DELETE FROM contacts WHERE id=$1 AND source='website'`, id)
	if e != nil {
		writeWebsiteJSON(w, 500, map[string]string{"error": "Could not delete enquiry"})
		return
	}
	if tag.RowsAffected() == 0 {
		writeWebsiteJSON(w, 404, map[string]string{"error": "Enquiry not found"})
		return
	}
	writeWebsiteJSON(w, 200, map[string]string{"message": "Enquiry deleted"})
}
func enquiryID(r *http.Request) (int64, error) {
	id, e := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if e != nil || id < 1 {
		return 0, pgx.ErrNoRows
	}
	return id, nil
}
