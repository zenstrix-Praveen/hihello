package website

import (
	"digital-card-platform/backend/internal/cards"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TemplateHandler struct {
	DB *pgxpool.Pool
}

type Template struct {
	ID              int64           `json:"id"`
	Name            string          `json:"name"`
	Slug            string          `json:"slug"`
	Description     string          `json:"description"`
	PreviewImageURL string          `json:"preview_image_url"`
	Category        string          `json:"category"`
	DesignSettings  json.RawMessage `json:"design_settings"`
	IsFeatured      bool            `json:"is_featured"`
	IsEnabled       bool            `json:"is_enabled"`
	SortOrder       int             `json:"sort_order"`
	UsageCount      int64           `json:"usage_count"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

func (h *TemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	search := "%" + strings.TrimSpace(r.URL.Query().Get("search")) + "%"
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	enabled := strings.TrimSpace(r.URL.Query().Get("enabled"))
	rows, err := h.DB.Query(
		r.Context(),
		`
        SELECT
            id,
            name,
            slug,
            description,
            preview_image_url,
            category,
            design_settings,
            is_featured,
            is_enabled,
            sort_order,
            created_at,
            updated_at,
            (SELECT COUNT(*) FROM digital_cards c WHERE c.template_id=website_templates.id)
        FROM website_templates
        WHERE ($1='' OR name ILIKE $2 OR slug ILIKE $2)
          AND ($3='' OR category=$3)
          AND ($4='' OR is_enabled=$4::boolean)
        ORDER BY sort_order ASC, id ASC
        `,
		strings.TrimSpace(r.URL.Query().Get("search")), search, category, enabled,
	)
	if err != nil {
		log.Printf("AdminTemplates query failed: %v", err)
		templateError(w, http.StatusInternalServerError, "Could not load templates")
		return
	}
	defer rows.Close()

	items := []Template{}

	for rows.Next() {
		var item Template

		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Slug,
			&item.Description,
			&item.PreviewImageURL,
			&item.Category,
			&item.DesignSettings,
			&item.IsFeatured,
			&item.IsEnabled,
			&item.SortOrder,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.UsageCount,
		); err != nil {
			log.Printf("AdminTemplates row scan failed: %v", err)
			templateError(w, http.StatusInternalServerError, "Could not read templates")
			return
		}

		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		log.Printf("AdminTemplates rows failed: %v", err)
		templateError(w, http.StatusInternalServerError, "Could not read templates")
		return
	}

	templateJSON(w, http.StatusOK, items)
}

func (h *TemplateHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := templateID(r)
	if err != nil {
		templateError(w, http.StatusBadRequest, "Invalid template ID")
		return
	}

	var item Template

	err = h.DB.QueryRow(
		r.Context(),
		`
        SELECT
            id,
            name,
            slug,
            description,
            preview_image_url,
            category,
            design_settings,
            is_featured,
            is_enabled,
            sort_order,
            created_at,
            updated_at,
            (SELECT COUNT(*) FROM digital_cards c WHERE c.template_id=website_templates.id)
        FROM website_templates
        WHERE id = $1
        `,
		id,
	).Scan(
		&item.ID,
		&item.Name,
		&item.Slug,
		&item.Description,
		&item.PreviewImageURL,
		&item.Category,
		&item.DesignSettings,
		&item.IsFeatured,
		&item.IsEnabled,
		&item.SortOrder,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.UsageCount,
	)

	if err != nil {
		templateError(w, http.StatusNotFound, "Template not found")
		return
	}

	templateJSON(w, http.StatusOK, item)
}

type TemplateCard struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Owner        string    `json:"owner"`
	Organization string    `json:"organization"`
	Company      string    `json:"company"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

func (h *TemplateHandler) Cards(w http.ResponseWriter, r *http.Request) {
	id, err := templateID(r)
	if err != nil {
		templateError(w, http.StatusBadRequest, "Invalid template ID")
		return
	}
	rows, err := h.DB.Query(r.Context(), `SELECT c.id,c.name,u.name,COALESCE(o.name,''),COALESCE(c.company_name,''),c.status,c.created_at
		FROM digital_cards c JOIN users u ON u.id=c.user_id LEFT JOIN organizations o ON o.id=c.organization_id
		WHERE c.template_id=$1 ORDER BY c.created_at DESC,c.id DESC`, id)
	if err != nil {
		templateError(w, http.StatusInternalServerError, "Could not load template cards")
		return
	}
	defer rows.Close()
	items := []TemplateCard{}
	for rows.Next() {
		var item TemplateCard
		if err := rows.Scan(&item.ID, &item.Name, &item.Owner, &item.Organization, &item.Company, &item.Status, &item.CreatedAt); err != nil {
			templateError(w, http.StatusInternalServerError, "Could not read template cards")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		templateError(w, http.StatusInternalServerError, "Could not read template cards")
		return
	}
	templateJSON(w, http.StatusOK, map[string]interface{}{"data": items, "usage_count": len(items)})
}

func (h *TemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input Template

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		templateError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	input.Name = strings.TrimSpace(input.Name)
	input.Slug = strings.TrimSpace(input.Slug)
	settings, _, settingsErr := cards.NormalizeDesignSettings(input.DesignSettings, "#2563EB")
	if settingsErr != nil {
		templateError(w, http.StatusBadRequest, settingsErr.Error())
		return
	}
	input.DesignSettings = settings

	if input.Name == "" || input.Slug == "" {
		templateError(w, http.StatusBadRequest, "Name and slug are required")
		return
	}

	var exists bool

	err := h.DB.QueryRow(
		r.Context(),
		`SELECT EXISTS(SELECT 1 FROM website_templates WHERE slug = $1)`,
		input.Slug,
	).Scan(&exists)

	if err != nil {
		templateError(w, http.StatusInternalServerError, "Could not validate slug")
		return
	}

	if exists {
		templateError(w, http.StatusConflict, "Template slug already exists")
		return
	}

	err = h.DB.QueryRow(
		r.Context(),
		`
        INSERT INTO website_templates (
            name,
            slug,
            description,
            preview_image_url,
            category,
            design_settings,
            is_featured,
            is_enabled,
            sort_order
        )
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
        RETURNING id
        `,
		input.Name,
		input.Slug,
		input.Description,
		input.PreviewImageURL,
		input.Category,
		input.DesignSettings,
		input.IsFeatured,
		input.IsEnabled,
		input.SortOrder,
	).Scan(&input.ID)

	if err != nil {
		templateError(w, http.StatusInternalServerError, "Could not create template")
		return
	}

	templateJSON(w, http.StatusCreated, input)
}

func (h *TemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := templateID(r)
	if err != nil {
		templateError(w, http.StatusBadRequest, "Invalid template ID")
		return
	}

	var input Template

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		templateError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	input.Name = strings.TrimSpace(input.Name)
	input.Slug = strings.TrimSpace(input.Slug)
	settings, _, settingsErr := cards.NormalizeDesignSettings(input.DesignSettings, "#2563EB")
	if settingsErr != nil {
		templateError(w, http.StatusBadRequest, settingsErr.Error())
		return
	}
	input.DesignSettings = settings

	if input.Name == "" || input.Slug == "" {
		templateError(w, http.StatusBadRequest, "Name and slug are required")
		return
	}

	var duplicate bool

	err = h.DB.QueryRow(
		r.Context(),
		`
        SELECT EXISTS(
            SELECT 1
            FROM website_templates
            WHERE slug = $1
            AND id <> $2
        )
        `,
		input.Slug,
		id,
	).Scan(&duplicate)

	if err != nil {
		templateError(w, http.StatusInternalServerError, "Could not validate slug")
		return
	}

	if duplicate {
		templateError(w, http.StatusConflict, "Template slug already exists")
		return
	}

	result, err := h.DB.Exec(
		r.Context(),
		`
        UPDATE website_templates
        SET
            name = $1,
            slug = $2,
            description = $3,
            preview_image_url = $4,
            category = $5,
            design_settings = $6,
            is_featured = $7,
            is_enabled = $8,
            sort_order = $9,
            updated_at = NOW()
        WHERE id = $10
        `,
		input.Name,
		input.Slug,
		input.Description,
		input.PreviewImageURL,
		input.Category,
		input.DesignSettings,
		input.IsFeatured,
		input.IsEnabled,
		input.SortOrder,
		id,
	)

	if err != nil {
		templateError(w, http.StatusInternalServerError, "Could not update template")
		return
	}

	if result.RowsAffected() == 0 {
		templateError(w, http.StatusNotFound, "Template not found")
		return
	}

	input.ID = id

	templateJSON(w, http.StatusOK, input)
}

func (h *TemplateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := templateID(r)
	if err != nil {
		templateError(w, http.StatusBadRequest, "Invalid template ID")
		return
	}

	result, err := h.DB.Exec(
		r.Context(),
		`DELETE FROM website_templates WHERE id = $1`,
		id,
	)

	if err != nil {
		templateError(w, http.StatusInternalServerError, "Could not delete template")
		return
	}

	if result.RowsAffected() == 0 {
		templateError(w, http.StatusNotFound, "Template not found")
		return
	}

	templateJSON(w, http.StatusOK, map[string]string{
		"message": "Template deleted successfully",
	})
}

func templateID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func templateJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func templateError(w http.ResponseWriter, status int, message string) {
	templateJSON(w, status, map[string]string{
		"error": message,
	})
}
