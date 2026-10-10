package cards

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/skip2/go-qrcode"
)

type publicCardStore interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

type PublicHandler struct{ DB publicCardStore }

const publicCardQuery = `SELECT c.id,c.user_id,jsonb_build_object(
 'id',c.id,'public_id',c.public_id,'name',c.name,'slug',c.slug,'job_title',COALESCE(c.job_title,''),
 'company_name',COALESCE(c.company_name,''),'phone',COALESCE(c.phone,''),'email',COALESCE(c.email,''),
 'website',CASE WHEN c.website ~* '^https?://[^/[:space:]]+' THEN c.website ELSE '' END,
 'address',COALESCE(c.address,''),
 'photo_url',CASE WHEN c.photo_url ~* '^https?://[^/[:space:]]+' THEN c.photo_url ELSE '' END,
 'logo_url',CASE WHEN c.logo_url ~* '^https?://[^/[:space:]]+' THEN c.logo_url ELSE '' END,
	'primary_color',c.primary_color,'design_settings',c.design_settings,'template_id',c.template_id,'template_name',COALESCE(t.name,''),
 'template_preview_image_url',CASE WHEN t.preview_image_url ~* '^https?://[^/[:space:]]+' THEN t.preview_image_url ELSE '' END,
 'social_links',COALESCE((SELECT jsonb_agg(jsonb_build_object('label',platform,'url',url) ORDER BY sort_order,id) FROM digital_card_social_links WHERE card_id=c.id AND url ~* '^https?://[^/[:space:]]+'),'[]'::jsonb),
 'custom_links',COALESCE((SELECT jsonb_agg(jsonb_build_object('label',label,'url',url) ORDER BY sort_order,id) FROM digital_card_custom_links WHERE card_id=c.id AND url ~* '^https?://[^/[:space:]]+'),'[]'::jsonb),
 'videos',COALESCE((SELECT jsonb_agg(jsonb_build_object('label',title,'url',url) ORDER BY sort_order,id) FROM digital_card_videos WHERE card_id=c.id AND url ~* '^https?://[^/[:space:]]+'),'[]'::jsonb),
 'published_at',c.published_at)
 FROM digital_cards c LEFT JOIN website_templates t ON t.id=c.template_id
 WHERE c.slug=$1 AND c.status='published'`

type publicCard struct {
	ID        int64           `json:"id"`
	UserID    int64           `json:"-"`
	Data      json.RawMessage `json:"-"`
	Name      string          `json:"name"`
	Slug      string          `json:"slug"`
	Title     string          `json:"job_title"`
	Company   string          `json:"company_name"`
	Phone     string          `json:"phone"`
	Email     string          `json:"email"`
	Website   string          `json:"website"`
	Address   string          `json:"address"`
	Published string          `json:"published_at"`
}

func (h *PublicHandler) load(r *http.Request) (publicCard, error) {
	var card publicCard
	err := h.DB.QueryRow(r.Context(), publicCardQuery, chi.URLParam(r, "slug")).Scan(&card.ID, &card.UserID, &card.Data)
	if err != nil {
		return card, err
	}
	if err := json.Unmarshal(card.Data, &card); err != nil {
		return card, err
	}
	return card, nil
}

func (h *PublicHandler) Get(w http.ResponseWriter, r *http.Request) {
	card, err := h.load(r)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeCardJSON(w, http.StatusNotFound, map[string]string{"error": "Card not available"})
		} else {
			writeCardJSON(w, http.StatusInternalServerError, map[string]string{"error": "Could not load card"})
		}
		return
	}
	// Public viewing should still work if analytics storage is temporarily unavailable.
	if r.URL.Query().Get("metadata") != "1" {
		_, _ = h.DB.Exec(r.Context(), `INSERT INTO analytics_events(event_type,card_id,user_id,metadata) VALUES('card_view',$1,$2,'{"source":"public_card"}'::jsonb)`, card.ID, card.UserID)
	}
	w.Header().Set("Cache-Control", "no-store")
	writeCardJSON(w, http.StatusOK, map[string]json.RawMessage{"data": card.Data})
}

type publicEventRequest struct {
	Type   string `json:"type"`
	Action string `json:"action"`
}

var publicSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func validPublicAction(eventType, action string) bool {
	if eventType == "contact_exchange" {
		return action == "save_contact"
	}
	if eventType != "link_click" {
		return false
	}
	switch action {
	case "call", "whatsapp", "email", "website", "map", "social", "custom", "video", "qr":
		return true
	case "share":
		return true
	default:
		return false
	}
}

func (h *PublicHandler) Event(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !publicSlugPattern.MatchString(slug) {
		http.NotFound(w, r)
		return
	}
	var in publicEventRequest
	if !decodeCustomerCard(w, r, &in) {
		return
	}
	if !validPublicAction(in.Type, in.Action) {
		writeCardJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid public card event"})
		return
	}
	var id, userID int64
	if err := h.DB.QueryRow(r.Context(), `SELECT id,user_id FROM digital_cards WHERE slug=$1 AND status='published'`, slug).Scan(&id, &userID); err != nil {
		http.NotFound(w, r)
		return
	}
	metadata, _ := json.Marshal(map[string]string{"action": in.Action, "source": "public_card"})
	// Analytics is best-effort and never blocks a public action.
	_, _ = h.DB.Exec(r.Context(), `INSERT INTO analytics_events(event_type,card_id,user_id,metadata) VALUES($1,$2,$3,$4::jsonb)`, in.Type, id, userID, string(metadata))
	w.WriteHeader(http.StatusNoContent)
}

func vCardEscape(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, ";", `\;`)
	value = strings.ReplaceAll(value, ",", `\,`)
	value = strings.ReplaceAll(value, "\r\n", `\n`)
	value = strings.ReplaceAll(value, "\r", `\n`)
	return strings.ReplaceAll(value, "\n", `\n`)
}

func (h *PublicHandler) VCard(w http.ResponseWriter, r *http.Request) {
	card, err := h.load(r)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Could not load contact", http.StatusInternalServerError)
		}
		return
	}
	var lines []string
	lines = append(lines, "BEGIN:VCARD", "VERSION:3.0", "FN:"+vCardEscape(card.Name))
	if card.Company != "" {
		lines = append(lines, "ORG:"+vCardEscape(card.Company))
	}
	if card.Title != "" {
		lines = append(lines, "TITLE:"+vCardEscape(card.Title))
	}
	if card.Phone != "" {
		lines = append(lines, "TEL;TYPE=CELL:"+vCardEscape(card.Phone))
	}
	if card.Email != "" {
		lines = append(lines, "EMAIL;TYPE=INTERNET:"+vCardEscape(card.Email))
	}
	if card.Website != "" {
		if u, e := url.ParseRequestURI(card.Website); e == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
			lines = append(lines, "URL:"+vCardEscape(card.Website))
		}
	}
	if card.Address != "" {
		lines = append(lines, "ADR;TYPE=WORK:;;"+vCardEscape(card.Address)+";;;;")
	}
	lines = append(lines, "END:VCARD", "")
	filename := card.Slug
	if !publicSlugPattern.MatchString(filename) {
		filename = fmt.Sprintf("contact-%d", card.ID)
	}
	w.Header().Set("Content-Type", "text/vcard; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`.vcf"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(strings.Join(lines, "\r\n")))
}

func (h *PublicHandler) QR(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	var cardID int64
	if err := h.DB.QueryRow(r.Context(), `SELECT id FROM digital_cards WHERE slug=$1 AND status='published'`, slug).Scan(&cardID); err != nil {
		http.NotFound(w, r)
		return
	}
	var token string
	if err := h.DB.QueryRow(r.Context(), `SELECT code_value FROM qr_codes WHERE card_id=$1 AND status='active' ORDER BY created_at DESC,id DESC LIMIT 1`, cardID).Scan(&token); err != nil {
		http.NotFound(w, r)
		return
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("API_PUBLIC_URL")), "/")
	if base == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		base = scheme + "://" + r.Host
	}
	image, err := qrcode.Encode(base+"/q/"+url.PathEscape(token), qrcode.Medium, 1024)
	if err != nil {
		http.Error(w, "Could not generate QR code", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(image)
}
