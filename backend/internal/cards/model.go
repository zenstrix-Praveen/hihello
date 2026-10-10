package cards

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

type DesignSettings struct {
	Theme           string   `json:"theme"`
	PrimaryColor    string   `json:"primary_color"`
	SecondaryColor  string   `json:"secondary_color"`
	BackgroundColor string   `json:"background_color"`
	TextColor       string   `json:"text_color"`
	AccentColor     string   `json:"accent_color"`
	FontFamily      string   `json:"font_family"`
	FontScale       float64  `json:"font_scale"`
	Layout          string   `json:"layout"`
	ButtonStyle     string   `json:"button_style"`
	BorderRadius    string   `json:"border_radius"`
	ProfileStyle    string   `json:"profile_style"`
	LogoPosition    string   `json:"logo_position"`
	SocialIconStyle string   `json:"social_icon_style"`
	SectionOrder    []string `json:"section_order"`
}

var designColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
var designThemes = map[string]bool{"ocean-blue": true, "royal-indigo": true, "emerald": true, "midnight": true, "sunset": true, "rose": true, "violet": true, "teal": true, "monochrome": true, "custom": true}
var designFonts = map[string]bool{"System Sans": true, "Arial": true, "Trebuchet MS": true, "Georgia": true}
var designLayouts = map[string]bool{"classic": true, "centered": true, "split": true, "hero": true, "minimal": true, "compact": true, "executive": true}

func DefaultDesignSettings(primary string) DesignSettings {
	if !designColorPattern.MatchString(primary) {
		primary = "#2563EB"
	}
	return DesignSettings{Theme: "ocean-blue", PrimaryColor: primary, SecondaryColor: "#4F46E5", BackgroundColor: "#FFFFFF", TextColor: "#0F172A", AccentColor: "#06B6D4", FontFamily: "System Sans", FontScale: 1, Layout: "classic", ButtonStyle: "rounded", BorderRadius: "large", ProfileStyle: "circle", LogoPosition: "top-right", SocialIconStyle: "pills", SectionOrder: []string{"about", "contact", "social", "links", "videos", "qr"}}
}

func NormalizeDesignSettings(raw json.RawMessage, legacyPrimary string) (json.RawMessage, DesignSettings, error) {
	design := DefaultDesignSettings(legacyPrimary)
	if len(raw) > 0 && string(raw) != "null" {
		defaults, _ := json.Marshal(design)
		var merged, supplied map[string]json.RawMessage
		if err := json.Unmarshal(defaults, &merged); err != nil {
			return nil, DesignSettings{}, fmt.Errorf("could not create default design settings")
		}
		if err := json.Unmarshal(raw, &supplied); err != nil || supplied == nil {
			return nil, DesignSettings{}, fmt.Errorf("design settings must be a JSON object")
		}
		for key, value := range supplied {
			if _, exists := merged[key]; exists {
				merged[key] = value
			}
		}
		combined, _ := json.Marshal(merged)
		if err := json.Unmarshal(combined, &design); err != nil {
			return nil, DesignSettings{}, fmt.Errorf("design settings must be valid")
		}
	}
	if !designThemes[design.Theme] || !designFonts[design.FontFamily] || !designLayouts[design.Layout] {
		return nil, DesignSettings{}, fmt.Errorf("select a supported theme, font, and card layout")
	}
	for _, value := range []string{design.PrimaryColor, design.SecondaryColor, design.BackgroundColor, design.TextColor, design.AccentColor} {
		if !designColorPattern.MatchString(value) {
			return nil, DesignSettings{}, fmt.Errorf("design colors must be six-digit hex values")
		}
	}
	if design.FontScale < 0.85 || design.FontScale > 1.25 {
		return nil, DesignSettings{}, fmt.Errorf("font scale must be between 0.85 and 1.25")
	}
	if !map[string]bool{"rounded": true, "pill": true, "square": true}[design.ButtonStyle] || !map[string]bool{"small": true, "medium": true, "large": true}[design.BorderRadius] || !map[string]bool{"circle": true, "square": true, "rounded": true}[design.ProfileStyle] || !map[string]bool{"top-right": true, "before-name": true, "below-name": true}[design.LogoPosition] || !map[string]bool{"pills": true, "monochrome": true, "branded": true}[design.SocialIconStyle] {
		return nil, DesignSettings{}, fmt.Errorf("one or more card style options are unsupported")
	}
	allowedSections := map[string]bool{"about": true, "contact": true, "social": true, "links": true, "videos": true, "qr": true}
	seen := map[string]bool{}
	for _, section := range design.SectionOrder {
		if !allowedSections[section] || seen[section] {
			return nil, DesignSettings{}, fmt.Errorf("card section order contains an unsupported or duplicate section")
		}
		seen[section] = true
	}
	encoded, err := json.Marshal(design)
	return encoded, design, err
}

type DigitalCard struct {
	ID             int64          `json:"id"`
	PublicID       string         `json:"public_id"`
	UserID         int64          `json:"user_id"`
	OrganizationID *int64         `json:"organization_id,omitempty"`
	Name           string         `json:"name"`
	Slug           string         `json:"slug"`
	JobTitle       string         `json:"job_title"`
	CompanyName    string         `json:"company_name"`
	Phone          string         `json:"phone"`
	Email          string         `json:"email"`
	Website        string         `json:"website"`
	Address        string         `json:"address"`
	PhotoURL       string         `json:"photo_url"`
	LogoURL        string         `json:"logo_url"`
	PrimaryColor   string         `json:"primary_color"`
	DesignSettings DesignSettings `json:"design_settings"`
	TemplateID     *int64         `json:"template_id,omitempty"`
	Status         string         `json:"status"`
	PublishedAt    *time.Time     `json:"published_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	SocialLinks    []LinkObject   `json:"social_links,omitempty"`
	CustomLinks    []LinkObject   `json:"custom_links,omitempty"`
	Videos         []LinkObject   `json:"videos,omitempty"`
}
