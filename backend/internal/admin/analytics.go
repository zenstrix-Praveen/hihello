package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AnalyticsHandler struct{ DB *pgxpool.Pool }

type analyticsRange struct {
	Start time.Time
	End   time.Time
	Type  string
}

var validAnalyticsEvents = map[string]bool{
	"card_view": true, "qr_scan": true, "contact_exchange": true,
	"link_click": true, "card_created": true, "user_registered": true, "nfc_tap": true,
}

func parseAnalyticsRange(r *http.Request) (analyticsRange, error) {
	q := r.URL.Query()
	now := time.Now()
	end := now
	start := now.AddDate(0, 0, -6)
	switch q.Get("range") {
	case "", "7d":
	case "today":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "30d":
		start = now.AddDate(0, 0, -29)
	case "custom":
		var err error
		start, err = time.Parse("2006-01-02", q.Get("start"))
		if err != nil {
			return analyticsRange{}, err
		}
		endDate, err := time.Parse("2006-01-02", q.Get("end"))
		if err != nil {
			return analyticsRange{}, err
		}
		end = endDate.Add(24*time.Hour - time.Nanosecond)
	default:
		return analyticsRange{}, strconv.ErrSyntax
	}
	if start.After(end) {
		return analyticsRange{}, strconv.ErrSyntax
	}
	typ := q.Get("event_type")
	if typ != "" && !validAnalyticsEvents[typ] {
		return analyticsRange{}, strconv.ErrSyntax
	}
	return analyticsRange{Start: start, End: end, Type: typ}, nil
}

func (h *AnalyticsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	rng, err := parseAnalyticsRange(r)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid date range or event type"})
		return
	}
	ctx := r.Context()
	out := make(map[string]int64)
	queries := []struct{ name, query string }{
		{"card_views", `SELECT COUNT(*) FROM analytics_events WHERE event_type='card_view' AND created_at BETWEEN $1 AND $2`},
		{"qr_scans", `SELECT COUNT(*) FROM analytics_events WHERE event_type='qr_scan' AND created_at BETWEEN $1 AND $2`},
		{"contact_exchanges", `SELECT COUNT(*) FROM analytics_events WHERE event_type='contact_exchange' AND created_at BETWEEN $1 AND $2`},
		{"link_clicks", `SELECT COUNT(*) FROM analytics_events WHERE event_type='link_click' AND created_at BETWEEN $1 AND $2`},
		{"active_cards", `SELECT COUNT(*) FROM digital_cards WHERE status='published'`},
		{"new_users", `SELECT COUNT(*) FROM users WHERE created_at BETWEEN $1 AND $2`},
		{"new_contacts", `SELECT COUNT(*) FROM contacts WHERE created_at BETWEEN $1 AND $2`},
		{"new_cards", `SELECT COUNT(*) FROM digital_cards WHERE created_at BETWEEN $1 AND $2`},
	}
	for _, item := range queries {
		var value int64
		query := h.DB.QueryRow(ctx, item.query, rng.Start, rng.End)
		if item.name == "active_cards" {
			query = h.DB.QueryRow(ctx, item.query)
		}
		if err := query.Scan(&value); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not load analytics summary"})
			return
		}
		out[item.name] = value
	}
	if out["card_views"] > 0 {
		out["conversion_rate"] = out["contact_exchanges"] * 10000 / out["card_views"]
	} else {
		out["conversion_rate"] = 0
	}
	writeJSON(w, 200, map[string]interface{}{"data": out, "conversion_rate_unit": "basis_points"})
}

type dailyAnalytics struct {
	Date             string `json:"date"`
	CardViews        int64  `json:"card_views"`
	QRScans          int64  `json:"qr_scans"`
	ContactExchanges int64  `json:"contact_exchanges"`
	LinkClicks       int64  `json:"link_clicks"`
}

func (h *AnalyticsHandler) Timeseries(w http.ResponseWriter, r *http.Request) {
	rng, err := parseAnalyticsRange(r)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid date range or event type"})
		return
	}
	filter := ""
	args := []interface{}{rng.Start, rng.End}
	if rng.Type != "" {
		filter = " AND event_type=$3"
		args = append(args, rng.Type)
	}
	rows, err := h.DB.Query(r.Context(), `SELECT date_trunc('day',created_at)::date,
	COUNT(*) FILTER (WHERE event_type='card_view'), COUNT(*) FILTER (WHERE event_type='qr_scan'),
	COUNT(*) FILTER (WHERE event_type='contact_exchange'), COUNT(*) FILTER (WHERE event_type='link_click')
	FROM analytics_events WHERE created_at BETWEEN $1 AND $2`+filter+`
	GROUP BY 1 ORDER BY 1`, args...)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load analytics timeseries"})
		return
	}
	defer rows.Close()
	data := []dailyAnalytics{}
	for rows.Next() {
		var item dailyAnalytics
		var date time.Time
		if err := rows.Scan(&date, &item.CardViews, &item.QRScans, &item.ContactExchanges, &item.LinkClicks); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read analytics timeseries"})
			return
		}
		item.Date = date.Format("2006-01-02")
		data = append(data, item)
	}
	if rows.Err() != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not read analytics timeseries"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": data})
}

type topAnalyticsCard struct {
	CardID           int64  `json:"card_id"`
	CardName         string `json:"card_name"`
	Owner            string `json:"owner"`
	Organization     string `json:"organization"`
	Views            int64  `json:"views"`
	QRScans          int64  `json:"qr_scans"`
	ContactExchanges int64  `json:"contact_exchanges"`
	LinkClicks       int64  `json:"link_clicks"`
}

func (h *AnalyticsHandler) TopCards(w http.ResponseWriter, r *http.Request) {
	rng, err := parseAnalyticsRange(r)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid date range or event type"})
		return
	}
	filter := ""
	args := []interface{}{rng.Start, rng.End}
	if rng.Type != "" {
		args = append(args, rng.Type)
		filter = " AND e.event_type=$3"
	}
	rows, err := h.DB.Query(r.Context(), `SELECT c.id,c.name,u.name,COALESCE(o.name,''),
	COUNT(*) FILTER (WHERE e.event_type='card_view'),COUNT(*) FILTER (WHERE e.event_type='qr_scan'),
	COUNT(*) FILTER (WHERE e.event_type='contact_exchange'),COUNT(*) FILTER (WHERE e.event_type='link_click')
	FROM analytics_events e JOIN digital_cards c ON c.id=e.card_id JOIN users u ON u.id=c.user_id
	LEFT JOIN organizations o ON o.id=c.organization_id WHERE e.created_at BETWEEN $1 AND $2`+filter+`
	GROUP BY c.id,c.name,u.name,o.name ORDER BY 5 DESC,6 DESC,7 DESC LIMIT 10`, args...)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load top cards"})
		return
	}
	defer rows.Close()
	data := []topAnalyticsCard{}
	for rows.Next() {
		var x topAnalyticsCard
		if err := rows.Scan(&x.CardID, &x.CardName, &x.Owner, &x.Organization, &x.Views, &x.QRScans, &x.ContactExchanges, &x.LinkClicks); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read top cards"})
			return
		}
		data = append(data, x)
	}
	if rows.Err() != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not read top cards"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": data})
}

type recentAnalytics struct {
	Timestamp    time.Time `json:"timestamp"`
	EventType    string    `json:"event_type"`
	Card         string    `json:"card"`
	User         string    `json:"user"`
	Organization string    `json:"organization"`
	Source       string    `json:"source"`
}

func (h *AnalyticsHandler) Recent(w http.ResponseWriter, r *http.Request) {
	rng, err := parseAnalyticsRange(r)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid date range or event type"})
		return
	}
	filter := ""
	args := []interface{}{rng.Start, rng.End}
	if rng.Type != "" {
		args = append(args, rng.Type)
		filter = " AND e.event_type=$3"
	}
	rows, err := h.DB.Query(r.Context(), `SELECT e.created_at,e.event_type,COALESCE(c.name,''),COALESCE(u.name,''),COALESCE(o.name,''),COALESCE(e.metadata->>'source','')
	FROM analytics_events e LEFT JOIN digital_cards c ON c.id=e.card_id LEFT JOIN users u ON u.id=COALESCE(e.user_id,c.user_id)
	LEFT JOIN organizations o ON o.id=c.organization_id WHERE e.created_at BETWEEN $1 AND $2`+filter+` ORDER BY e.created_at DESC LIMIT 50`, args...)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load recent activity"})
		return
	}
	defer rows.Close()
	data := []recentAnalytics{}
	for rows.Next() {
		var x recentAnalytics
		if err := rows.Scan(&x.Timestamp, &x.EventType, &x.Card, &x.User, &x.Organization, &x.Source); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read recent activity"})
			return
		}
		data = append(data, x)
	}
	if rows.Err() != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not read recent activity"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": data})
}
