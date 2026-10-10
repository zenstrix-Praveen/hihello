// Package currency defines the application's selectable plan currencies.
package currency

import (
	_ "embed"
	"encoding/xml"
	"strconv"
	"strings"
)

// Currency is the canonical plan currency definition. Exponent is the number
// of fractional decimal places accepted in a price/minor-unit conversion.
type Currency struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Exponent int    `json:"exponent"`
}

var legacy = [...]Currency{
	{Code: "USD", Name: "US Dollar", Symbol: "$", Exponent: 2},
	{Code: "INR", Name: "Indian Rupee", Symbol: "₹", Exponent: 2},
	{Code: "AED", Name: "UAE Dirham", Symbol: "د.إ", Exponent: 2},
	{Code: "SAR", Name: "Saudi Riyal", Symbol: "ر.س", Exponent: 2},
	{Code: "QAR", Name: "Qatari Riyal", Symbol: "ر.ق", Exponent: 2},
	{Code: "KWD", Name: "Kuwaiti Dinar", Symbol: "د.ك", Exponent: 3},
	{Code: "BHD", Name: "Bahraini Dinar", Symbol: "د.ب", Exponent: 3},
	{Code: "OMR", Name: "Omani Rial", Symbol: "ر.ع.", Exponent: 3},
	{Code: "EGP", Name: "Egyptian Pound", Symbol: "E£", Exponent: 2},
	{Code: "JOD", Name: "Jordanian Dinar", Symbol: "د.ا", Exponent: 3},
	{Code: "EUR", Name: "Euro", Symbol: "€", Exponent: 2},
	{Code: "GBP", Name: "British Pound", Symbol: "£", Exponent: 2},
}

func Catalog() []Currency { return append([]Currency{}, selectable[:]...) }

// Codes returns an independent copy so callers cannot change the catalog.
func Codes() []string {
	result := make([]string, 0, len(selectable))
	for _, item := range selectable {
		result = append(result, item.Code)
	}
	return result
}

func Normalize(code string) string { return strings.ToUpper(strings.TrimSpace(code)) }

func Supported(code string) bool {
	code = Normalize(code)
	for _, allowed := range selectable {
		if code == allowed.Code {
			return true
		}
	}
	return false
}

func Exponent(code string) (int, bool) {
	code = Normalize(code)
	for _, item := range selectable {
		if item.Code == code {
			return item.Exponent, true
		}
	}
	return 0, false
}

// ISO List One from SIX, the ISO 4217 maintenance agency. Numeric minor units
// are included; metals/testing codes with N.A. precision are not chargeable.
// Refresh iso4217.xml from the documented official source and review the diff.
//
//go:embed iso4217.xml
var registryXML []byte
var selectable = loadRegistry()

func loadRegistry() []Currency {
	var doc struct {
		Entries []struct {
			Code  string `xml:"Ccy"`
			Name  string `xml:"CcyNm"`
			Minor string `xml:"CcyMnrUnts"`
		} `xml:"CcyTbl>CcyNtry"`
	}
	if err := xml.Unmarshal(registryXML, &doc); err != nil {
		panic("Invalid embedded ISO currency registry")
	}
	result := append([]Currency{}, legacy[:]...)
	seen := map[string]bool{}
	for _, c := range result {
		seen[c.Code] = true
	}
	for _, e := range doc.Entries {
		exp, err := strconv.Atoi(e.Minor)
		if e.Code == "" || err != nil || seen[e.Code] {
			continue
		}
		seen[e.Code] = true
		result = append(result, Currency{Code: e.Code, Name: strings.TrimSpace(e.Name), Symbol: e.Code, Exponent: exp})
	}
	return result
}
