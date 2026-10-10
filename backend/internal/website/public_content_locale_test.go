package website

import "testing"

func TestHasUntranslatedWebsiteText(t *testing.T) {
	english := map[string]interface{}{
		"page_title":       "Plans",
		"cta_url":          "/register",
		"navigation_items": []interface{}{map[string]interface{}{"label": "Home", "url": "/"}, map[string]interface{}{"label": "Pricing", "url": "/pricing"}},
	}
	localized := map[string]interface{}{
		"page_title":       "الخطط",
		"cta_url":          "/register",
		"navigation_items": []interface{}{map[string]interface{}{"label": "الرئيسية", "url": "/"}},
	}
	if !hasUntranslatedWebsiteText(english, localized, "") {
		t.Fatal("missing translation in nested CMS child should trigger fallback")
	}
	localized["navigation_items"] = []interface{}{
		map[string]interface{}{"label": "الرئيسية", "url": "/"},
		map[string]interface{}{"label": "الأسعار", "url": "/pricing"},
	}
	if hasUntranslatedWebsiteText(english, localized, "") {
		t.Fatal("translated labels and unchanged URLs should not trigger fallback")
	}
}
