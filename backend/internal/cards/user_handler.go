package cards

import (
	"context"
	"crypto/rand"
	"digital-card-platform/backend/internal/customer"
	"digital-card-platform/backend/internal/middleware"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/skip2/go-qrcode"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type TransactionalEmailSender interface {
	SendEmail(context.Context, string, string, string) error
}

type UserHandler struct {
	DB          *pgxpool.Pool
	EmailSender TransactionalEmailSender
}

// Explicit customer projection, scoped in every query before records are returned.
const userCardSelect = `SELECT jsonb_build_object(
 'id',c.id,'public_id',c.public_id,'name',c.name,'slug',c.slug,'job_title',COALESCE(c.job_title,''),
 'company_name',COALESCE(c.company_name,''),'email',COALESCE(c.email,''),'phone',COALESCE(c.phone,''),
 'website',COALESCE(c.website,''),'address',COALESCE(c.address,''),'photo_url',COALESCE(c.photo_url,''),'logo_url',COALESCE(c.logo_url,''),
 'primary_color',c.primary_color,'template_id',c.template_id,'template_name',COALESCE(t.name,''),'template_preview_image_url',COALESCE(t.preview_image_url,''),
	'design_settings',c.design_settings,
 'status',c.status,'published_at',c.published_at,'created_at',c.created_at,'updated_at',c.updated_at,
 'views',(SELECT COUNT(*) FROM analytics_events WHERE card_id=c.id AND event_type='card_view'),
 'qr_scans',(SELECT COALESCE(SUM(q.scan_count),0) FROM qr_codes q WHERE q.card_id=c.id),
 'nfc_taps',(SELECT COALESCE(SUM(n.tap_count),0) FROM nfc_tags n WHERE n.card_id=c.id),
 'qr_count',(SELECT COUNT(*) FROM qr_codes WHERE card_id=c.id),
 'nfc_count',(SELECT COUNT(*) FROM nfc_tags WHERE card_id=c.id),
 'social_links',COALESCE((SELECT jsonb_agg(jsonb_build_object('label',platform,'url',url) ORDER BY sort_order,id) FROM digital_card_social_links WHERE card_id=c.id),'[]'::jsonb),
 'custom_links',COALESCE((SELECT jsonb_agg(jsonb_build_object('label',label,'url',url) ORDER BY sort_order,id) FROM digital_card_custom_links WHERE card_id=c.id),'[]'::jsonb),
 'videos',COALESCE((SELECT jsonb_agg(jsonb_build_object('label',title,'url',url) ORDER BY sort_order,id) FROM digital_card_videos WHERE card_id=c.id),'[]'::jsonb))
 FROM digital_cards c LEFT JOIN website_templates t ON t.id=c.template_id WHERE c.user_id=$1`

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	page, limit := 1, 20
	if n, e := strconv.Atoi(r.URL.Query().Get("page")); e == nil && n > 0 && n <= 1000000 {
		page = n
	}
	if n, e := strconv.Atoi(r.URL.Query().Get("limit")); e == nil && n > 0 && n <= 100 {
		limit = n
	}
	uid := r.Context().Value(middleware.UserIDKey)
	var total int64
	if err := h.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM digital_cards WHERE user_id=$1`, uid).Scan(&total); err != nil {
		jsonError(w, 500, "Could not count cards")
		return
	}
	rows, err := h.DB.Query(r.Context(), userCardSelect+` ORDER BY c.updated_at DESC,c.id DESC LIMIT $2 OFFSET $3`, uid, limit, (page-1)*limit)
	if err != nil {
		jsonError(w, 500, "Could not load cards")
		return
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var card json.RawMessage
		if err := rows.Scan(&card); err != nil {
			jsonError(w, 500, "Could not read cards")
			return
		}
		out = append(out, card)
	}
	if rows.Err() != nil {
		jsonError(w, 500, "Could not read cards")
		return
	}
	writeCardJSON(w, 200, map[string]any{"data": out, "pagination": map[string]any{"page": page, "limit": limit, "total": total, "pages": (total + int64(limit) - 1) / int64(limit)}})
}

func cardID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		jsonError(w, 400, "Invalid card id")
		return 0, false
	}
	return id, true
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := cardID(w, r)
	if !ok {
		return
	}
	var card json.RawMessage
	err := h.DB.QueryRow(r.Context(), userCardSelect+` AND c.id=$2`, r.Context().Value(middleware.UserIDKey), id).Scan(&card)
	if errors.Is(err, pgx.ErrNoRows) {
		jsonError(w, 404, "Card not found")
		return
	}
	if err != nil {
		jsonError(w, 500, "Could not load card")
		return
	}
	writeCardJSON(w, 200, map[string]any{"data": card})
}

func decodeCustomerCard(w http.ResponseWriter, r *http.Request, in any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024))
	if decoder.Decode(in) != nil || decoder.Decode(new(any)) != io.EOF {
		jsonError(w, 400, "Invalid card request")
		return false
	}
	return true
}

func customerStatus(status string) bool {
	return status == "draft" || status == "published" || status == "archived"
}

func validCustomerURL(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	u, err := url.ParseRequestURI(value)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func validateCustomerURLs(in adminCardInput) error {
	for _, field := range []struct {
		value string
		max   int
	}{
		{in.Name, 150}, {in.Slug, 180}, {in.JobTitle, 150}, {in.CompanyName, 200}, {in.Phone, 50}, {in.Email, 255}, {in.Website, 2048}, {in.Address, 4000}, {in.PhotoURL, 2048}, {in.LogoURL, 2048},
	} {
		if len(field.value) > field.max {
			return errors.New("one or more card fields are too long")
		}
	}
	for _, value := range []string{in.Website, in.PhotoURL, in.LogoURL} {
		if !validCustomerURL(value) {
			return errors.New("URLs must use http or https")
		}
	}
	for index, group := range [][]LinkObject{in.SocialLinks, in.CustomLinks, in.Videos} {
		labelMax := []int{50, 150, 200}[index]
		for _, item := range group {
			label := item["label"]
			if label == "" {
				label = item["platform"]
			}
			if label == "" {
				label = item["title"]
			}
			if len(label) > labelMax {
				return errors.New("one or more link labels are too long")
			}
			if !validCustomerURL(item["url"]) {
				return errors.New("link URLs must use http or https")
			}
		}
	}
	return nil
}

func cardWriteError(w http.ResponseWriter, err error) {
	fmt.Printf("CARD SAVE ERROR: %T: %v\n", err, err)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		fmt.Printf(
			"POSTGRES CARD ERROR: code=%s constraint=%s table=%s column=%s detail=%s message=%s\n",
			pgErr.Code,
			pgErr.ConstraintName,
			pgErr.TableName,
			pgErr.ColumnName,
			pgErr.Detail,
			pgErr.Message,
		)

		if pgErr.Code == "23505" {
			jsonError(w, 409, "This card slug is already in use")
			return
		}
	}

	jsonError(w, 500, "Could not save card")
}
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) { h.save(w, r, false) }
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) { h.save(w, r, true) }

func (h *UserHandler) save(w http.ResponseWriter, r *http.Request, update bool) {
	var id int64
	if update {
		var ok bool
		id, ok = cardID(w, r)
		if !ok {
			return
		}
	}
	var in adminCardInput
	if !decodeCustomerCard(w, r, &in) {
		return
	}
	// Ownership and organization membership are never assigned from customer input.
	uid, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok || uid < 1 {
		jsonError(w, 401, "A valid user session is required")
		return
	}
	in.UserID = uid
	in.OrganizationID = nil
	if in.Status == "" {
		in.Status = "draft"
	}
	if !customerStatus(in.Status) {
		jsonError(w, 400, "Status must be draft, published, or archived")
		return
	}
	if err := validateCustomerURLs(in); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	settings, normalizedDesign, err := NormalizeDesignSettings(in.DesignSettings, in.PrimaryColor)
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	in.DesignSettings = settings
	in.PrimaryColor = normalizedDesign.PrimaryColor
	var cardEntitlements customer.EntitlementResult
	if !update {
		cardEntitlements, err = (&customer.Handler{DB: h.DB}).ResolveEntitlements(r.Context(), uid)
		if err != nil {
			jsonError(w, 500, "Could not validate plan limits")
			return
		}
	}
	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		jsonError(w, 500, "Could not save card")
		return
	}
	defer tx.Rollback(r.Context())
	if !update {
		if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1::text,0))`, fmt.Sprintf("plan-cards:%d", uid)); err != nil {
			jsonError(w, 500, "Could not validate plan limits")
			return
		}
		var usage int
		if err = tx.QueryRow(r.Context(), `SELECT COUNT(*) FROM digital_cards WHERE user_id=$1 AND status<>'archived'`, uid).Scan(&usage); err != nil {
			jsonError(w, 500, "Could not validate plan limits")
			return
		}
		if !cardEntitlements.CanUse("digital_cards", usage) {
			jsonError(w, 403, "Your plan's digital card limit has been reached")
			return
		}
	}
	if update {
		var currentStatus string
		err = tx.QueryRow(r.Context(), `SELECT status FROM digital_cards WHERE id=$1 AND user_id=$2 FOR UPDATE`, id, uid).Scan(&currentStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			jsonError(w, 404, "Card not found")
			return
		}
		if err != nil {
			jsonError(w, 500, "Could not load card")
			return
		}
		if currentStatus == "suspended" {
			jsonError(w, 403, "A suspended card cannot be edited")
			return
		}
	}
	if err := validateAdminCard(r.Context(), h.DB, &in); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	if update {
		_, err = tx.Exec(r.Context(), `UPDATE digital_cards SET template_id=$1,name=$2,slug=$3,job_title=$4,company_name=$5,phone=$6,email=$7,website=$8,address=$9,photo_url=$10,logo_url=$11,primary_color=$12,design_settings=$13,status=$14,published_at=CASE WHEN $14='published' THEN COALESCE(published_at,NOW()) ELSE published_at END,updated_at=NOW() WHERE id=$15 AND user_id=$16`, in.TemplateID, in.Name, in.Slug, in.JobTitle, in.CompanyName, in.Phone, in.Email, in.Website, in.Address, in.PhotoURL, in.LogoURL, in.PrimaryColor, in.DesignSettings, in.Status, id, uid)
	} else {
		err = tx.QueryRow(r.Context(), `INSERT INTO digital_cards(user_id,template_id,name,slug,job_title,company_name,phone,email,website,address,photo_url,logo_url,primary_color,design_settings,status,published_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15::varchar,CASE WHEN $15::varchar='published' THEN NOW() ELSE NULL END) RETURNING id`, uid, in.TemplateID, in.Name, in.Slug, in.JobTitle, in.CompanyName, in.Phone, in.Email, in.Website, in.Address, in.PhotoURL, in.LogoURL, in.PrimaryColor, in.DesignSettings, in.Status).Scan(&id)
	}
	if err != nil {
		cardWriteError(w, err)
		return
	}
	if err = saveLinks(r.Context(), tx, id, in); err != nil {
		jsonError(w, 400, "Could not save card links")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		jsonError(w, 500, "Could not save card")
		return
	}
	status := 201
	if update {
		status = 200
	}
	writeCardJSON(w, status, map[string]any{"data": map[string]any{"id": id}})
}

func (h *UserHandler) Status(w http.ResponseWriter, r *http.Request) {
	id, ok := cardID(w, r)
	if !ok {
		return
	}
	var in struct {
		Status string `json:"status"`
	}
	if !decodeCustomerCard(w, r, &in) {
		return
	}
	if !customerStatus(in.Status) {
		jsonError(w, 400, "Status must be draft, published, or archived")
		return
	}
	tag, err := h.DB.Exec(r.Context(), `UPDATE digital_cards SET status=$1,published_at=CASE WHEN $2='published' THEN COALESCE(published_at,NOW()) ELSE published_at END,updated_at=NOW() WHERE id=$3 AND user_id=$4 AND status<>'suspended'`, in.Status, in.Status, id, r.Context().Value(middleware.UserIDKey))
	if err != nil {
		jsonError(w, 500, "Could not update card status")
		return
	}
	if tag.RowsAffected() == 0 {
		jsonError(w, 404, "Card not found or unavailable")
		return
	}
	writeCardJSON(w, 200, map[string]string{"message": "Card status updated"})
}

// CreateQR creates or returns the active QR record for one published card owned by this user.
func (h *UserHandler) CreateQR(w http.ResponseWriter, r *http.Request) {
	id, ok := cardID(w, r)
	if !ok {
		return
	}
	uid, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok || uid < 1 {
		jsonError(w, 401, "A valid user session is required")
		return
	}
	qrEntitlements, err := (&customer.Handler{DB: h.DB}).ResolveEntitlements(r.Context(), uid)
	if err != nil {
		jsonError(w, 500, "Could not validate plan limits")
		return
	}
	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		jsonError(w, 500, "Could not prepare QR code")
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1::text,0))`, fmt.Sprintf("plan-qr:%d", uid)); err != nil {
		jsonError(w, 500, "Could not validate plan limits")
		return
	}
	var slug, name, status string
	err = tx.QueryRow(r.Context(), `SELECT slug,name,status FROM digital_cards WHERE id=$1 AND user_id=$2 FOR UPDATE`, id, uid).Scan(&slug, &name, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		jsonError(w, 404, "Card not found")
		return
	}
	if err != nil {
		jsonError(w, 500, "Could not load card")
		return
	}
	if status != "published" {
		jsonError(w, 409, "Publish this card before generating its QR code")
		return
	}
	var qrID int64
	var token string
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_WEB_URL")), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("FRONTEND_URL")), "/")
	}
	if base == "" {
		base = "http://localhost:3000"
	}
	destination := base + "/c/" + url.PathEscape(slug)
	err = tx.QueryRow(r.Context(), `SELECT id,code_value FROM qr_codes WHERE card_id=$1 AND status='active' ORDER BY created_at,id LIMIT 1 FOR UPDATE`, id).Scan(&qrID, &token)
	if errors.Is(err, pgx.ErrNoRows) {
		var usage int
		if err = tx.QueryRow(r.Context(), `SELECT COUNT(*) FROM qr_codes WHERE user_id=$1 AND status='active'`, uid).Scan(&usage); err != nil {
			jsonError(w, 500, "Could not validate plan limits")
			return
		}
		if !qrEntitlements.CanUse("qr_codes", usage) {
			jsonError(w, 403, "Your plan's QR code limit has been reached")
			return
		}
		bytes := make([]byte, 32)
		if _, err = rand.Read(bytes); err != nil {
			jsonError(w, 500, "Could not generate QR code")
			return
		}
		token = base64.RawURLEncoding.EncodeToString(bytes)
		err = tx.QueryRow(r.Context(), `INSERT INTO qr_codes(code_value,card_id,user_id,destination_url,label,status,updated_at) VALUES($1,$2,$3,$4,$5,'active',NOW()) RETURNING id`, token, id, uid, destination, fmt.Sprintf("Card %d QR", id)).Scan(&qrID)
		if err != nil {
			jsonError(w, 500, "Could not create QR code")
			return
		}
	} else if err != nil {
		jsonError(w, 500, "Could not load QR code")
		return
	} else {
		if _, err = tx.Exec(r.Context(), `UPDATE qr_codes SET destination_url=$1,updated_at=NOW() WHERE id=$2`, destination, qrID); err != nil {
			jsonError(w, 500, "Could not update QR destination")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		jsonError(w, 500, "Could not save QR code")
		return
	}
	apiBase := strings.TrimRight(strings.TrimSpace(os.Getenv("API_PUBLIC_URL")), "/")
	if apiBase == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		apiBase = scheme + "://" + r.Host
	}
	qrPNG, err := qrcode.Encode(apiBase+"/q/"+url.PathEscape(token), qrcode.Medium, 1024)
	if err != nil {
		jsonError(w, 500, "Could not generate QR image")
		return
	}
	qrImage := "data:image/png;base64," + base64.StdEncoding.EncodeToString(qrPNG)
	writeCardJSON(w, 200, map[string]any{"data": map[string]any{"id": qrID, "card_id": id, "card_name": name, "card_slug": slug, "code_value": token, "status": "active", "qr_image_path": "/public/cards/" + url.PathEscape(slug) + "/qr", "qr_image": qrImage}})
}

// ShareEmail sends the recipient only the published card URL and optional sender-written note.
func (h *UserHandler) ShareEmail(w http.ResponseWriter, r *http.Request) {
	id, ok := cardID(w, r)
	if !ok {
		return
	}
	uid, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok || uid < 1 {
		jsonError(w, 401, "A valid user session is required")
		return
	}
	var input struct {
		Email   string `json:"email"`
		Message string `json:"message"`
	}
	if !decodeCustomerCard(w, r, &input) {
		return
	}
	input.Email = strings.TrimSpace(input.Email)
	input.Message = strings.TrimSpace(input.Message)
	if len(input.Message) > 500 || strings.ContainsAny(input.Email, "\r\n") {
		jsonError(w, 400, "Enter a valid recipient email and a message under 500 characters")
		return
	}
	var ownerName, slug, name, title, company, status string
	err := h.DB.QueryRow(r.Context(), `SELECT u.name,c.slug,c.name,COALESCE(c.job_title,''),COALESCE(c.company_name,''),c.status FROM digital_cards c JOIN users u ON u.id=c.user_id WHERE c.id=$1 AND c.user_id=$2`, id, uid).Scan(&ownerName, &slug, &name, &title, &company, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		jsonError(w, 404, "Card not found")
		return
	}
	if err != nil {
		jsonError(w, 500, "Could not load card")
		return
	}
	if status != "published" {
		jsonError(w, 409, "Publish this card before sharing it")
		return
	}
	if h.EmailSender == nil {
		jsonError(w, 503, "Email sharing is not configured")
		return
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_WEB_URL")), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("FRONTEND_URL")), "/")
	}
	if base == "" {
		base = "http://localhost:3000"
	}
	parsed, parseErr := url.ParseRequestURI(base)
	if parseErr != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		jsonError(w, 503, "Public card links are not configured")
		return
	}
	publicURL := base + "/c/" + url.PathEscape(slug)
	senderName := strings.Join(strings.Fields(ownerName), " ")
	if senderName == "" {
		senderName = "A Dbizcard user"
	}
	subject := senderName + " shared a digital business card with you"
	body := fmt.Sprintf("Hello,\n\n%s has shared their digital business card with you.\n\n", senderName)
	if input.Message != "" {
		body += input.Message + "\n\n"
	}
	body += fmt.Sprintf("%s\n", name)
	if title != "" || company != "" {
		body += strings.Trim(strings.Join([]string{title, company}, " · "), " ·") + "\n"
	}
	body += fmt.Sprintf("\nView the card:\n%s\n\nDbizcard", publicURL)
	if err = h.EmailSender.SendEmail(r.Context(), input.Email, subject, body); err != nil {
		jsonError(w, 502, "Could not send card email. Please try again.")
		return
	}
	writeCardJSON(w, 200, map[string]string{"message": "Card email sent."})
}
