package currency

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)

type Conversion struct {
	Amount          float64   `json:"amount"`
	From            string    `json:"from"`
	To              string    `json:"to"`
	Rate            float64   `json:"rate"`
	ConvertedAmount float64   `json:"converted_amount"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type RateProvider interface {
	Rate(context.Context, string, string) (float64, time.Time, error)
}

type Frankfurter struct{ Client *http.Client }

func (f Frankfurter) Rate(ctx context.Context, from, to string) (float64, time.Time, error) {
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	u := "https://api.frankfurter.app/latest?from=" + url.QueryEscape(from) + "&to=" + url.QueryEscape(to)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, time.Time{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, time.Time{}, fmt.Errorf("provider status %d", resp.StatusCode)
	}
	var body struct {
		Date  string             `json:"date"`
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, time.Time{}, err
	}
	rate, ok := body.Rates[to]
	if !ok || !finitePositive(rate) {
		return 0, time.Time{}, fmt.Errorf("unsupported currency")
	}
	updated, err := time.Parse("2006-01-02", body.Date)
	if err != nil {
		updated = time.Now().UTC()
	}
	return rate, updated.UTC(), nil
}

type Handler struct{ Provider RateProvider }

// List returns the canonical currency definitions used for subscription plans.
func (h Handler) List(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusOK, map[string]any{"data": Catalog()})
}

func (h Handler) Convert(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	amount, err := strconv.ParseFloat(q.Get("amount"), 64)
	if err != nil || !finitePositive(amount) {
		respond(w, http.StatusBadRequest, map[string]string{"error": "Amount must be a number greater than zero"})
		return
	}
	from, to := strings.ToUpper(strings.TrimSpace(q.Get("from"))), strings.ToUpper(strings.TrimSpace(q.Get("to")))
	if !currencyCode.MatchString(from) || !currencyCode.MatchString(to) {
		respond(w, http.StatusBadRequest, map[string]string{"error": "Currency codes must be three letters"})
		return
	}
	if from == to {
		respond(w, http.StatusOK, Conversion{Amount: amount, From: from, To: to, Rate: 1, ConvertedAmount: amount, UpdatedAt: time.Now().UTC()})
		return
	}
	if h.Provider == nil {
		respond(w, http.StatusServiceUnavailable, map[string]string{"error": "Exchange-rate service is temporarily unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	rate, updated, err := h.Provider.Rate(ctx, from, to)
	if err != nil {
		if strings.Contains(err.Error(), "unsupported currency") {
			respond(w, http.StatusUnprocessableEntity, map[string]string{"error": "One or both currencies are not supported"})
			return
		}
		respond(w, http.StatusServiceUnavailable, map[string]string{"error": "Exchange-rate service is temporarily unavailable"})
		return
	}
	respond(w, http.StatusOK, Conversion{Amount: amount, From: from, To: to, Rate: rate, ConvertedAmount: amount * rate, UpdatedAt: updated})
}

func finitePositive(v float64) bool { return v > 0 && !math.IsInf(v, 0) && !math.IsNaN(v) }
func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
