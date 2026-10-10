// Package pricing resolves explicit plan prices. It never performs FX conversion.
package pricing

import (
	"digital-card-platform/backend/internal/currency"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

type Price struct {
	Currency  string `json:"currency"`
	Country   string `json:"country"`
	Period    string `json:"billing_period"`
	Amount    string `json:"amount"`
	Precision *int   `json:"decimal_precision,omitempty"`
	Rounding  string `json:"rounding,omitempty"`
	TaxBPS    int64  `json:"tax_bps"`
}
type Config struct {
	Prices    []Price  `json:"prices"`
	Providers []string `json:"providers"`
}
type Quote struct {
	Currency  string   `json:"currency"`
	Country   string   `json:"country"`
	Period    string   `json:"billing_period"`
	Subtotal  string   `json:"subtotal"`
	Tax       string   `json:"tax"`
	Total     string   `json:"total"`
	Minor     int64    `json:"amount_minor"`
	Providers []string `json:"providers"`
}

var decimal = regexp.MustCompile(`^(0|[1-9][0-9]*)(?:\.([0-9]+))?$`)
var countryCode = regexp.MustCompile(`^[A-Z]{2}$`)

func Country(s string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s != "" && !countryCode.MatchString(s) {
		return "", errors.New("Country must be an ISO two-letter code")
	}
	return s, nil
}
func Minor(amount, code string) (int64, error) {
	exp, ok := currency.Exponent(code)
	if !ok {
		return 0, errors.New("Unknown currency")
	}
	m := decimal.FindStringSubmatch(strings.TrimSpace(amount))
	if m == nil {
		return 0, errors.New("Invalid decimal amount")
	}
	f := m[2]
	if len(f) > exp {
		if strings.Trim(f[exp:], "0") != "" {
			return 0, errors.New("Amount exceeds currency precision")
		}
		f = f[:exp]
	}
	n, ok := new(big.Int).SetString(m[1]+f+strings.Repeat("0", exp-len(f)), 10)
	if !ok || !n.IsInt64() {
		return 0, errors.New("Amount is too large")
	}
	return n.Int64(), nil
}
func Major(minor int64, code string) string {
	exp, _ := currency.Exponent(code)
	n := fmt.Sprintf("%d", minor)
	if exp == 0 {
		return n
	}
	if len(n) <= exp {
		n = strings.Repeat("0", exp-len(n)+1) + n
	}
	return n[:len(n)-exp] + "." + n[len(n)-exp:]
}
func Parse(raw []byte) (Config, error) {
	c := Config{Prices: []Price{}, Providers: []string{"razorpay"}}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &c); err != nil {
			return c, err
		}
	}
	seen := map[string]bool{}
	for i := range c.Prices {
		p := &c.Prices[i]
		p.Currency = currency.Normalize(p.Currency)
		var err error
		p.Country, err = Country(p.Country)
		if err != nil {
			return c, err
		}
		if p.Period != "monthly" && p.Period != "yearly" && p.Period != "lifetime" {
			return c, errors.New("Invalid price billing period")
		}
		priceMinor, priceErr := Minor(p.Amount, p.Currency)
		if priceErr != nil {
			err = priceErr
		}
		if priceMinor == 0 && priceErr == nil {
			return c, errors.New("Configured paid prices must be greater than zero")
		}
		if err != nil {
			return c, err
		}
		exp, _ := currency.Exponent(p.Currency)
		if p.Precision != nil && (*p.Precision < 0 || *p.Precision > exp) {
			return c, errors.New("Decimal precision must not exceed ISO currency precision")
		}
		if p.Rounding != "" && p.Rounding != "reject" && p.Rounding != "half_up" && p.Rounding != "down" && p.Rounding != "up" {
			return c, errors.New("Invalid rounding rule")
		}
		if p.TaxBPS < 0 || p.TaxBPS > 10000 {
			return c, errors.New("Tax basis points must be between 0 and 10000")
		}
		key := p.Currency + ":" + p.Country + ":" + p.Period
		if seen[key] {
			return c, errors.New("Duplicate country/currency/period price")
		}
		seen[key] = true
	}
	seen = map[string]bool{}
	for _, p := range c.Providers {
		if p == "" || len(p) > 30 || seen[p] {
			return c, errors.New("Invalid or duplicate payment provider")
		}
		seen[p] = true
	}
	if c.Prices == nil {
		c.Prices = []Price{}
	}
	if c.Providers == nil {
		c.Providers = []string{}
	}
	return c, nil
}

// Country-specific prices outrank general currency prices; base price applies
// only to its stored currency and billing period. Tax rounds half up to ISO units.
func Resolve(c Config, baseCurrency, baseAmount, basePeriod, selected, country, period string) (Quote, error) {
	selected = currency.Normalize(selected)
	if selected == "" {
		selected = currency.Normalize(baseCurrency)
	}
	if period == "" {
		period = basePeriod
	}
	country, err := Country(country)
	if err != nil {
		return Quote{}, err
	}
	amount := ""
	bps := int64(0)
	score := -1
	var precision *int
	rounding := ""
	if selected == currency.Normalize(baseCurrency) && period == basePeriod {
		amount = baseAmount
		score = 0
	}
	for _, p := range c.Prices {
		if p.Currency != selected || p.Period != period {
			continue
		}
		s := 1
		if p.Country != "" {
			if p.Country != country {
				continue
			}
			s = 2
		}
		if s > score {
			amount = p.Amount
			bps = p.TaxBPS
			score = s
			precision = p.Precision
			rounding = p.Rounding
		}
	}
	if score < 0 {
		return Quote{}, errors.New("No configured price for the selected currency, country and billing period")
	}
	minor, err := Minor(amount, selected)
	if err != nil {
		return Quote{}, err
	}
	if precision != nil {
		exp, _ := currency.Exponent(selected)
		quantum := int64(1)
		for i := *precision; i < exp; i++ {
			quantum *= 10
		}
		rem := minor % quantum
		if rem != 0 {
			switch rounding {
			case "half_up":
				if rem >= quantum/2 {
					if minor > int64(^uint64(0)>>1)-(quantum-rem) {
						return Quote{}, errors.New("Rounded amount is too large")
					}
					minor += quantum - rem
				} else {
					minor -= rem
				}
			case "down":
				minor -= rem
			case "up":
				if minor > int64(^uint64(0)>>1)-(quantum-rem) {
					return Quote{}, errors.New("Rounded amount is too large")
				}
				minor += quantum - rem
			default:
				return Quote{}, errors.New("Price exceeds configured decimal precision")
			}
		}
	}
	tax := new(big.Int).Mul(big.NewInt(minor), big.NewInt(bps))
	tax.Add(tax, big.NewInt(5000))
	tax.Div(tax, big.NewInt(10000))
	total := new(big.Int).Add(big.NewInt(minor), tax)
	if !total.IsInt64() {
		return Quote{}, errors.New("Total is too large")
	}
	return Quote{Currency: selected, Country: country, Period: period, Subtotal: Major(minor, selected), Tax: Major(tax.Int64(), selected), Total: Major(total.Int64(), selected), Minor: total.Int64(), Providers: c.Providers}, nil
}
