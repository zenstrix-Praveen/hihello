package cards

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminHandler struct{ DB *pgxpool.Pool }

type adminCard struct {
	DigitalCard
	OwnerName        string `json:"owner_name"`
	OwnerEmail       string `json:"owner_email"`
	OrganizationName string `json:"organization_name"`
	TemplateName     string `json:"template_name"`
	TemplatePreview  string `json:"template_preview_image_url"`
	QRCount          int64  `json:"qr_count"`
	QRActiveCount    int64  `json:"qr_active_count"`
	NFCCount         int64  `json:"nfc_count"`
	NFCStatus        string `json:"nfc_status"`
	Views            int64  `json:"views"`
	QRScans          int64  `json:"qr_scans"`
	NFCTaps          int64  `json:"nfc_taps"`
	ContactExchanges int64  `json:"contact_exchanges"`
	LinkClicks       int64  `json:"link_clicks"`
}

func (h *AdminHandler) List(w http.ResponseWriter, r *http.Request) {
	page, limit := 1, 20
	if v, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && v > 0 {
		page = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = v
	}
	if limit > 100 {
		limit = 100
	}
	search := "%" + strings.TrimSpace(r.URL.Query().Get("search")) + "%"
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status == "expired" {
		status = "archived"
	}
	templateID := strings.TrimSpace(r.URL.Query().Get("template_id"))
	organizationID := strings.TrimSpace(r.URL.Query().Get("organization_id"))
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	where := `($1='' OR c.status=$1) AND ($3='' OR c.template_id::text=$3) AND ($4='' OR c.organization_id::text=$4) AND ($5='' OR c.user_id::text=$5) AND (c.name ILIKE $2 OR c.slug ILIKE $2 OR COALESCE(c.company_name,'') ILIKE $2 OR COALESCE(c.email,'') ILIKE $2 OR u.name ILIKE $2 OR u.email ILIKE $2)`
	var total int64
	if err := h.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM digital_cards c JOIN users u ON u.id=c.user_id WHERE `+where, status, search, templateID, organizationID, userID).Scan(&total); err != nil {
		jsonError(w, 500, "Could not count cards")
		return
	}
	rows, err := h.DB.Query(r.Context(), `SELECT c.id,c.public_id::text,c.user_id,c.organization_id,c.name,c.slug,COALESCE(c.job_title,''),COALESCE(c.company_name,''),COALESCE(c.phone,''),COALESCE(c.email,''),COALESCE(c.website,''),COALESCE(c.address,''),COALESCE(c.photo_url,''),COALESCE(c.logo_url,''),c.primary_color,c.template_id,c.status,c.published_at,c.created_at,c.updated_at,u.name,u.email,COALESCE(o.name,''),COALESCE(t.name,''),COALESCE(t.preview_image_url,''), (SELECT COUNT(*) FROM qr_codes q WHERE q.card_id=c.id),(SELECT COUNT(*) FROM qr_codes q WHERE q.card_id=c.id AND q.status='active'),(SELECT COUNT(*) FROM nfc_tags n WHERE n.card_id=c.id),(SELECT COALESCE(MIN(n.status),'') FROM nfc_tags n WHERE n.card_id=c.id),(SELECT COUNT(*) FROM analytics_events e WHERE e.card_id=c.id AND e.event_type='card_view'),(SELECT COUNT(*) FROM analytics_events e WHERE e.card_id=c.id AND e.event_type='qr_scan'),(SELECT COUNT(*) FROM analytics_events e WHERE e.card_id=c.id AND e.event_type='nfc_tap'),(SELECT COUNT(*) FROM analytics_events e WHERE e.card_id=c.id AND e.event_type='contact_exchange'),(SELECT COUNT(*) FROM analytics_events e WHERE e.card_id=c.id AND e.event_type='link_click') FROM digital_cards c JOIN users u ON u.id=c.user_id LEFT JOIN organizations o ON o.id=c.organization_id LEFT JOIN website_templates t ON t.id=c.template_id WHERE `+where+` ORDER BY c.created_at DESC LIMIT $6 OFFSET $7`, status, search, templateID, organizationID, userID, limit, (page-1)*limit)
	if err != nil {
		jsonError(w, 500, "Could not load cards")
		return
	}
	defer rows.Close()
	out := make([]adminCard, 0)
	for rows.Next() {
		var c adminCard
		if err := rows.Scan(&c.ID, &c.PublicID, &c.UserID, &c.OrganizationID, &c.Name, &c.Slug, &c.JobTitle, &c.CompanyName, &c.Phone, &c.Email, &c.Website, &c.Address, &c.PhotoURL, &c.LogoURL, &c.PrimaryColor, &c.TemplateID, &c.Status, &c.PublishedAt, &c.CreatedAt, &c.UpdatedAt, &c.OwnerName, &c.OwnerEmail, &c.OrganizationName, &c.TemplateName, &c.TemplatePreview, &c.QRCount, &c.QRActiveCount, &c.NFCCount, &c.NFCStatus, &c.Views, &c.QRScans, &c.NFCTaps, &c.ContactExchanges, &c.LinkClicks); err != nil {
			jsonError(w, 500, "Could not read cards")
			return
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		jsonError(w, 500, "Could not read cards")
		return
	}
	writeCardJSON(w, 200, map[string]interface{}{"data": out, "pagination": map[string]int{"page": page, "limit": limit, "total": int(total), "pages": int((total + int64(limit) - 1) / int64(limit))}})
}

func (h *AdminHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		jsonError(w, 400, "Invalid card id")
		return
	}
	var c adminCard
	err = h.DB.QueryRow(r.Context(), `SELECT c.id,c.public_id::text,c.user_id,c.organization_id,c.name,c.slug,COALESCE(c.job_title,''),COALESCE(c.company_name,''),COALESCE(c.phone,''),COALESCE(c.email,''),COALESCE(c.website,''),COALESCE(c.address,''),COALESCE(c.photo_url,''),COALESCE(c.logo_url,''),c.primary_color,c.template_id,c.status,c.published_at,c.created_at,c.updated_at,u.name,u.email,COALESCE(o.name,''),COALESCE(t.name,''),COALESCE(t.preview_image_url,''),(SELECT COUNT(*) FROM qr_codes q WHERE q.card_id=c.id),(SELECT COUNT(*) FROM qr_codes q WHERE q.card_id=c.id AND q.status='active'),(SELECT COUNT(*) FROM nfc_tags n WHERE n.card_id=c.id),(SELECT COALESCE(MIN(n.status),'') FROM nfc_tags n WHERE n.card_id=c.id),(SELECT COUNT(*) FROM analytics_events e WHERE e.card_id=c.id AND e.event_type='card_view'),(SELECT COUNT(*) FROM analytics_events e WHERE e.card_id=c.id AND e.event_type='qr_scan'),(SELECT COUNT(*) FROM analytics_events e WHERE e.card_id=c.id AND e.event_type='nfc_tap'),(SELECT COUNT(*) FROM analytics_events e WHERE e.card_id=c.id AND e.event_type='contact_exchange'),(SELECT COUNT(*) FROM analytics_events e WHERE e.card_id=c.id AND e.event_type='link_click') FROM digital_cards c JOIN users u ON u.id=c.user_id LEFT JOIN organizations o ON o.id=c.organization_id LEFT JOIN website_templates t ON t.id=c.template_id WHERE c.id=$1`, id).Scan(&c.ID, &c.PublicID, &c.UserID, &c.OrganizationID, &c.Name, &c.Slug, &c.JobTitle, &c.CompanyName, &c.Phone, &c.Email, &c.Website, &c.Address, &c.PhotoURL, &c.LogoURL, &c.PrimaryColor, &c.TemplateID, &c.Status, &c.PublishedAt, &c.CreatedAt, &c.UpdatedAt, &c.OwnerName, &c.OwnerEmail, &c.OrganizationName, &c.TemplateName, &c.TemplatePreview, &c.QRCount, &c.QRActiveCount, &c.NFCCount, &c.NFCStatus, &c.Views, &c.QRScans, &c.NFCTaps, &c.ContactExchanges, &c.LinkClicks)
	if err != nil {
		jsonError(w, 404, "Card not found")
		return
	}
	if c.SocialLinks, err = queryLinkObjects(r.Context(), h.DB, `SELECT platform,url FROM digital_card_social_links WHERE card_id=$1 ORDER BY sort_order,id`, id); err != nil {
		jsonError(w, 500, "Could not load social links")
		return
	}
	if c.CustomLinks, err = queryLinkObjects(r.Context(), h.DB, `SELECT label,url FROM digital_card_custom_links WHERE card_id=$1 ORDER BY sort_order,id`, id); err != nil {
		jsonError(w, 500, "Could not load custom links")
		return
	}
	if c.Videos, err = queryLinkObjects(r.Context(), h.DB, `SELECT COALESCE(title,''),url FROM digital_card_videos WHERE card_id=$1 ORDER BY sort_order,id`, id); err != nil {
		jsonError(w, 500, "Could not load videos")
		return
	}
	writeCardJSON(w, 200, map[string]interface{}{"data": c})
}

type adminCardInput struct {
	UserID         int64           `json:"user_id"`
	OrganizationID *int64          `json:"organization_id"`
	TemplateID     *int64          `json:"template_id"`
	Name           string          `json:"name"`
	Slug           string          `json:"slug"`
	JobTitle       string          `json:"job_title"`
	CompanyName    string          `json:"company_name"`
	Phone          string          `json:"phone"`
	Email          string          `json:"email"`
	Website        string          `json:"website"`
	Address        string          `json:"address"`
	PhotoURL       string          `json:"photo_url"`
	LogoURL        string          `json:"logo_url"`
	PrimaryColor   string          `json:"primary_color"`
	DesignSettings json.RawMessage `json:"design_settings"`
	Status         string          `json:"status"`
	SocialLinks    []LinkObject    `json:"social_links"`
	CustomLinks    []LinkObject    `json:"custom_links"`
	Videos         []LinkObject    `json:"videos"`
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func validateAdminCard(ctx context.Context, db *pgxpool.Pool, in *adminCardInput) error {
	in.Name, in.Slug, in.JobTitle, in.CompanyName = strings.TrimSpace(in.Name), strings.TrimSpace(in.Slug), strings.TrimSpace(in.JobTitle), strings.TrimSpace(in.CompanyName)
	in.Email, in.PrimaryColor, in.Status = strings.TrimSpace(in.Email), strings.TrimSpace(in.PrimaryColor), strings.TrimSpace(in.Status)
	if in.UserID < 1 || in.Name == "" || len(in.Name) > 150 || !slugPattern.MatchString(in.Slug) || len(in.Slug) > 180 {
		return fmt.Errorf("owner, card name, and a valid lowercase slug are required")
	}
	if in.Email != "" && (!strings.Contains(in.Email, "@") || len(in.Email) > 255) {
		return fmt.Errorf("enter a valid email address")
	}
	if in.PrimaryColor == "" {
		in.PrimaryColor = "#1267E8"
	}
	if !colorPattern.MatchString(in.PrimaryColor) {
		return fmt.Errorf("primary color must be a six-digit hex color")
	}
	if len(in.DesignSettings) > 0 {
		settings, normalized, err := NormalizeDesignSettings(in.DesignSettings, in.PrimaryColor)
		if err != nil {
			return err
		}
		in.DesignSettings = settings
		in.PrimaryColor = normalized.PrimaryColor
	} else {
		in.DesignSettings = json.RawMessage(`{}`)
	}
	if in.Status == "" {
		in.Status = "draft"
	}
	if in.Status != "draft" && in.Status != "published" && in.Status != "suspended" && in.Status != "archived" {
		return fmt.Errorf("status must be draft, published, suspended, or archived")
	}
	if in.TemplateID == nil {
		return fmt.Errorf("a template is required")
	}
	var exists bool
	if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1)`, in.UserID).Scan(&exists); err != nil || !exists {
		return fmt.Errorf("selected owner does not exist")
	}
	if in.OrganizationID != nil {
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM organizations WHERE id=$1)`, *in.OrganizationID).Scan(&exists); err != nil || !exists {
			return fmt.Errorf("selected organization does not exist")
		}
	}
	if in.TemplateID != nil {
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM website_templates WHERE id=$1 AND is_enabled=true)`, *in.TemplateID).Scan(&exists); err != nil || !exists {
			return fmt.Errorf("selected template is unavailable")
		}
	}
	for _, group := range [][]LinkObject{in.SocialLinks, in.CustomLinks, in.Videos} {
		if len(group) > 100 {
			return fmt.Errorf("a card can contain at most 100 links per section")
		}
		for _, item := range group {
			if strings.TrimSpace(item["url"]) == "" {
				return fmt.Errorf("link URLs are required")
			}
		}
	}
	return nil
}

func (h *AdminHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in adminCardInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		jsonError(w, 400, "Invalid card request")
		return
	}
	if err := validateAdminCard(r.Context(), h.DB, &in); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		jsonError(w, 500, "Could not create card")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	var id int64
	var publicID string
	err = tx.QueryRow(r.Context(), `INSERT INTO digital_cards(user_id,organization_id,template_id,name,slug,job_title,company_name,phone,email,website,address,photo_url,logo_url,primary_color,design_settings,status,published_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,CASE WHEN $16='published' THEN NOW() ELSE NULL END) RETURNING id,public_id::text`, in.UserID, in.OrganizationID, in.TemplateID, in.Name, in.Slug, in.JobTitle, in.CompanyName, in.Phone, in.Email, in.Website, in.Address, in.PhotoURL, in.LogoURL, in.PrimaryColor, in.DesignSettings, in.Status).Scan(&id, &publicID)
	if err != nil {
		jsonError(w, 409, "Could not create card; the slug may already be in use")
		return
	}
	if err = saveLinks(r.Context(), tx, id, in); err != nil {
		jsonError(w, 400, "Could not save card links")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		jsonError(w, 500, "Could not create card")
		return
	}
	writeCardJSON(w, 201, map[string]interface{}{"data": map[string]interface{}{"id": id, "public_id": publicID}})
}

func (h *AdminHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		jsonError(w, 400, "Invalid card id")
		return
	}
	var in adminCardInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		jsonError(w, 400, "Invalid card request")
		return
	}
	if err = validateAdminCard(r.Context(), h.DB, &in); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		jsonError(w, 500, "Could not update card")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	tag, err := tx.Exec(r.Context(), `UPDATE digital_cards SET user_id=$1,organization_id=$2,template_id=$3,name=$4,slug=$5,job_title=$6,company_name=$7,phone=$8,email=$9,website=$10,address=$11,photo_url=$12,logo_url=$13,primary_color=$14,design_settings=CASE WHEN $15='{}'::jsonb THEN design_settings ELSE $15::jsonb END,status=$16,published_at=CASE WHEN $16='published' THEN COALESCE(published_at,NOW()) ELSE published_at END,updated_at=NOW() WHERE id=$17`, in.UserID, in.OrganizationID, in.TemplateID, in.Name, in.Slug, in.JobTitle, in.CompanyName, in.Phone, in.Email, in.Website, in.Address, in.PhotoURL, in.LogoURL, in.PrimaryColor, in.DesignSettings, in.Status, id)
	if err != nil {
		jsonError(w, 409, "Could not update card; the slug may already be in use")
		return
	}
	if tag.RowsAffected() == 0 {
		jsonError(w, 404, "Card not found")
		return
	}
	if err = saveLinks(r.Context(), tx, id, in); err != nil {
		jsonError(w, 400, "Could not save card links")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		jsonError(w, 500, "Could not update card")
		return
	}
	writeCardJSON(w, 200, map[string]string{"message": "Card updated"})
}

func saveLinks(ctx context.Context, tx pgx.Tx, id int64, in adminCardInput) error {
	for _, spec := range []struct {
		table, label string
		rows         []LinkObject
	}{
		{"digital_card_social_links", "platform", in.SocialLinks},
		{"digital_card_custom_links", "label", in.CustomLinks},
		{"digital_card_videos", "title", in.Videos},
	} {
		if _, err := tx.Exec(ctx, `DELETE FROM `+spec.table+` WHERE card_id=$1`, id); err != nil {
			return err
		}
		for order, item := range spec.rows {
			label := strings.TrimSpace(item["label"])
			if label == "" {
				label = strings.TrimSpace(item[spec.label])
			}
			if spec.table == "digital_card_videos" {
				if _, err := tx.Exec(ctx, `INSERT INTO digital_card_videos(card_id,title,url,sort_order) VALUES($1,$2,$3,$4)`, id, label, strings.TrimSpace(item["url"]), order); err != nil {
					return err
				}
			} else if spec.table == "digital_card_social_links" {
				if _, err := tx.Exec(ctx, `INSERT INTO digital_card_social_links(card_id,platform,url,sort_order) VALUES($1,$2,$3,$4)`, id, label, strings.TrimSpace(item["url"]), order); err != nil {
					return err
				}
			} else if _, err := tx.Exec(ctx, `INSERT INTO digital_card_custom_links(card_id,label,url,sort_order) VALUES($1,$2,$3,$4)`, id, label, strings.TrimSpace(item["url"]), order); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *AdminHandler) Status(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		jsonError(w, 400, "Invalid card id")
		return
	}
	var in struct {
		Status string `json:"status"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		jsonError(w, 400, "Invalid status request")
		return
	}
	if in.Status != "draft" && in.Status != "published" && in.Status != "suspended" && in.Status != "archived" {
		jsonError(w, 400, "Status must be draft, published, suspended, or archived")
		return
	}
	tag, err := h.DB.Exec(r.Context(), `UPDATE digital_cards SET status=$1,published_at=CASE WHEN $1='published' THEN COALESCE(published_at,NOW()) ELSE published_at END,updated_at=NOW() WHERE id=$2`, in.Status, id)
	if err != nil {
		jsonError(w, 500, "Could not update card status")
		return
	}
	if tag.RowsAffected() == 0 {
		jsonError(w, 404, "Card not found")
		return
	}
	writeCardJSON(w, 200, map[string]string{"message": "Card status updated"})
}

func (h *AdminHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		jsonError(w, 400, "Invalid card id")
		return
	}
	var refs int64
	if err = h.DB.QueryRow(r.Context(), `SELECT (SELECT COUNT(*) FROM qr_codes WHERE card_id=$1)+(SELECT COUNT(*) FROM nfc_tags WHERE card_id=$1)+(SELECT COUNT(*) FROM analytics_events WHERE card_id=$1)`, id).Scan(&refs); err != nil {
		jsonError(w, 500, "Could not check card references")
		return
	}
	if refs > 0 {
		jsonError(w, 409, "Card has linked QR, NFC, or analytics history; archive it instead of deleting")
		return
	}
	tag, err := h.DB.Exec(r.Context(), `DELETE FROM digital_cards WHERE id=$1`, id)
	if err != nil {
		jsonError(w, 409, "Card cannot be deleted while it has related records")
		return
	}
	if tag.RowsAffected() == 0 {
		jsonError(w, 404, "Card not found")
		return
	}
	writeCardJSON(w, 200, map[string]string{"message": "Card deleted"})
}

func (h *AdminHandler) SetTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		jsonError(w, 400, "Invalid card id")
		return
	}
	var input struct {
		TemplateID *int64 `json:"template_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		jsonError(w, 400, "Invalid request")
		return
	}
	result, err := h.DB.Exec(r.Context(), `UPDATE digital_cards SET template_id=$1,updated_at=NOW() WHERE id=$2`, input.TemplateID, id)
	if err != nil {
		jsonError(w, 400, "Could not assign template")
		return
	}
	if result.RowsAffected() == 0 {
		jsonError(w, 404, "Card not found")
		return
	}
	writeCardJSON(w, 200, map[string]string{"message": "Card template updated"})
}

type LinkObject map[string]string

func queryLinkObjects(ctx context.Context, db *pgxpool.Pool, query string, id int64) ([]LinkObject, error) {
	rows, err := db.Query(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]LinkObject, 0)
	for rows.Next() {
		var label, url string
		if err := rows.Scan(&label, &url); err != nil {
			return nil, err
		}
		items = append(items, LinkObject{"label": label, "url": url})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func writeCardJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func jsonError(w http.ResponseWriter, status int, msg string) {
	writeCardJSON(w, status, map[string]string{"error": msg})
}
