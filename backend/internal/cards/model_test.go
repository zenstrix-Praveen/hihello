package cards

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeDesignSettingsDefaultsLegacyCards(t *testing.T) {
	raw, design, err := NormalizeDesignSettings(nil, "#123456")
	if err != nil {
		t.Fatal(err)
	}
	if design.PrimaryColor != "#123456" || design.FontFamily != "System Sans" || design.Layout != "classic" {
		t.Fatalf("unexpected defaults: %+v", design)
	}
	var persisted DesignSettings
	if err := json.Unmarshal(raw, &persisted); err != nil || persisted.PrimaryColor != design.PrimaryColor {
		t.Fatalf("normalized settings were not persisted: %+v, %v", persisted, err)
	}
}

func TestNormalizeDesignSettingsSupportsCustomization(t *testing.T) {
	input := json.RawMessage(`{"theme":"custom","primary_color":"#123456","font_family":"Georgia","layout":"executive","section_order":["contact","about"]}`)
	_, settings, err := NormalizeDesignSettings(input, "#2563EB")
	if err != nil {
		t.Fatal(err)
	}
	if settings.PrimaryColor != "#123456" || settings.FontFamily != "Georgia" || settings.Layout != "executive" || settings.SectionOrder[0] != "contact" {
		t.Fatalf("custom design settings were not retained: %+v", settings)
	}
}

func TestNormalizeDesignSettingsRejectsCSSAndUnsafeValues(t *testing.T) {
	for _, input := range []string{
		`{"primary_color":"red"}`,
		`{"font_family":"url(https://example.invalid)"}`,
		`{"layout":"<script>"}`,
		`{"font_scale":3}`,
		`{"section_order":["about","about"]}`,
	} {
		t.Run(strings.ReplaceAll(input, "/", "_"), func(t *testing.T) {
			if _, _, err := NormalizeDesignSettings(json.RawMessage(input), "#2563EB"); err == nil {
				t.Fatalf("expected unsafe design settings to be rejected: %s", input)
			}
		})
	}
}
