package admin

import (
	"net/http"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DashboardHandler struct{ DB *pgxpool.Pool }

type dashboardMetrics struct {
	TotalOrganizations  int64 `json:"total_organizations"`
	TotalUsers          int64 `json:"total_users"`
	ActiveUsers         int64 `json:"active_users"`
	TotalCards          int64 `json:"total_cards"`
	PublishedCards      int64 `json:"published_cards"`
	DraftCards          int64 `json:"draft_cards"`
	TotalContacts       int64 `json:"total_contacts"`
	TotalQRCodes        int64 `json:"total_qr_codes"`
	ActiveQRCodes       int64 `json:"active_qr_codes"`
	TotalNFCTags        int64 `json:"total_nfc_tags"`
	ActiveNFCTags       int64 `json:"active_nfc_tags"`
	TotalQRScans        int64 `json:"total_qr_scans"`
	TotalNFCTaps        int64 `json:"total_nfc_taps"`
	TotalTemplates      int64 `json:"total_templates"`
	ActiveSubscriptions int64 `json:"active_subscriptions"`
	NewUsersToday       int64 `json:"new_users_today"`
	CardsCreatedToday   int64 `json:"cards_created_today"`
	ContactsToday       int64 `json:"contacts_today"`
}

type dashboardPoint struct {
	Date  string `json:"date"`
	Value int64  `json:"value"`
}
type dashboardCard struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Owner            string `json:"owner"`
	Organization     string `json:"organization"`
	Views            int64  `json:"views"`
	QRScans          int64  `json:"qr_scans"`
	NFCTaps          int64  `json:"nfc_taps"`
	ContactExchanges int64  `json:"contact_exchanges"`
	Status           string `json:"status"`
}
type dashboardActivity struct {
	Event     string    `json:"event"`
	EventType string    `json:"event_type"`
	User      string    `json:"user"`
	Card      string    `json:"card"`
	Timestamp time.Time `json:"timestamp"`
}

func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var m dashboardMetrics
	err := h.DB.QueryRow(ctx, `SELECT
	(SELECT COUNT(*) FROM organizations), (SELECT COUNT(*) FROM users), (SELECT COUNT(*) FROM users WHERE status='active'),
	(SELECT COUNT(*) FROM digital_cards),
	(SELECT COUNT(*) FROM digital_cards WHERE status='published'),
	(SELECT COUNT(*) FROM digital_cards WHERE status='draft'),
	(SELECT COUNT(*) FROM contacts), (SELECT COUNT(*) FROM qr_codes),
	(SELECT COUNT(*) FROM qr_codes WHERE status='active'), (SELECT COUNT(*) FROM nfc_tags),
	(SELECT COUNT(*) FROM nfc_tags WHERE status='active'),
	(SELECT COALESCE(SUM(scan_count),0) FROM qr_codes),
	(SELECT COALESCE(SUM(tap_count),0) FROM nfc_tags),
	(SELECT COUNT(*) FROM website_templates),
	(SELECT COUNT(*) FROM subscriptions WHERE status='active'),
	(SELECT COUNT(*) FROM users WHERE created_at >= CURRENT_DATE),
	(SELECT COUNT(*) FROM digital_cards WHERE created_at >= CURRENT_DATE),
	(SELECT COUNT(*) FROM contacts WHERE created_at >= CURRENT_DATE)`).
		Scan(&m.TotalOrganizations, &m.TotalUsers, &m.ActiveUsers, &m.TotalCards, &m.PublishedCards, &m.DraftCards, &m.TotalContacts, &m.TotalQRCodes, &m.ActiveQRCodes, &m.TotalNFCTags, &m.ActiveNFCTags, &m.TotalQRScans, &m.TotalNFCTaps, &m.TotalTemplates, &m.ActiveSubscriptions, &m.NewUsersToday, &m.CardsCreatedToday, &m.ContactsToday)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load dashboard metrics"})
		return
	}

	days := 7
	if r.URL.Query().Get("range") == "30d" {
		days = 30
	}
	var dbToday string
	if err := h.DB.QueryRow(ctx, `SELECT CURRENT_DATE::text`).Scan(&dbToday); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load dashboard date range"})
		return
	}
	today, err := time.Parse("2006-01-02", dbToday)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load dashboard date range"})
		return
	}
	series := map[string][]dashboardPoint{"card_views": {}, "qr_scans": {}, "nfc_taps": {}, "contact_exchanges": {}}
	for key, eventType := range map[string]string{"card_views": "card_view", "qr_scans": "qr_scan", "nfc_taps": "nfc_tap", "contact_exchanges": "contact_exchange"} {
		rows, e := h.DB.Query(ctx, `SELECT created_at::date,COUNT(*) FROM analytics_events WHERE event_type=$1 AND created_at >= CURRENT_DATE-($2::int-1) GROUP BY created_at::date ORDER BY created_at::date`, eventType, days)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not load dashboard analytics"})
			return
		}
		counts := map[string]int64{}
		for rows.Next() {
			var d time.Time
			var n int64
			if e = rows.Scan(&d, &n); e != nil {
				rows.Close()
				writeJSON(w, 500, map[string]string{"error": "Could not read dashboard analytics"})
				return
			}
			counts[d.Format("2006-01-02")] = n
		}
		if e = rows.Err(); e != nil {
			rows.Close()
			writeJSON(w, 500, map[string]string{"error": "Could not read dashboard analytics"})
			return
		}
		rows.Close()
		for i := days - 1; i >= 0; i-- {
			d := today.AddDate(0, 0, -i).Format("2006-01-02")
			series[key] = append(series[key], dashboardPoint{Date: d, Value: counts[d]})
		}
	}

	topRows, err := h.DB.Query(ctx, `SELECT c.id,c.name,u.name,COALESCE(o.name,''),c.status,
	COUNT(e.id) FILTER(WHERE e.event_type='card_view'),COUNT(e.id) FILTER(WHERE e.event_type='qr_scan'),COUNT(e.id) FILTER(WHERE e.event_type='nfc_tap'),COUNT(e.id) FILTER(WHERE e.event_type='contact_exchange')
	FROM digital_cards c JOIN users u ON u.id=c.user_id LEFT JOIN organizations o ON o.id=c.organization_id
	LEFT JOIN analytics_events e ON e.card_id=c.id GROUP BY c.id,u.name,o.name ORDER BY 6 DESC,7 DESC,8 DESC,9 DESC LIMIT 10`)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load top digital cards"})
		return
	}
	top := []dashboardCard{}
	for topRows.Next() {
		var x dashboardCard
		if err = topRows.Scan(&x.ID, &x.Name, &x.Owner, &x.Organization, &x.Status, &x.Views, &x.QRScans, &x.NFCTaps, &x.ContactExchanges); err != nil {
			topRows.Close()
			writeJSON(w, 500, map[string]string{"error": "Could not read top digital cards"})
			return
		}
		top = append(top, x)
	}
	if err = topRows.Err(); err != nil {
		topRows.Close()
		writeJSON(w, 500, map[string]string{"error": "Could not read top digital cards"})
		return
	}
	topRows.Close()

	activity := []dashboardActivity{}
	rows, err := h.DB.Query(ctx, `SELECT e.event_type,COALESCE(u.name,''),COALESCE(c.name,''),e.created_at FROM analytics_events e LEFT JOIN digital_cards c ON c.id=e.card_id LEFT JOIN users u ON u.id=COALESCE(e.user_id,c.user_id) ORDER BY e.created_at DESC LIMIT 30`)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load recent activity"})
		return
	}
	for rows.Next() {
		var a dashboardActivity
		if err = rows.Scan(&a.EventType, &a.User, &a.Card, &a.Timestamp); err != nil {
			rows.Close()
			writeJSON(w, 500, map[string]string{"error": "Could not read recent activity"})
			return
		}
		a.Event = activityLabel(a.EventType)
		activity = append(activity, a)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		writeJSON(w, 500, map[string]string{"error": "Could not read recent activity"})
		return
	}
	rows.Close()
	adminRows, err := h.DB.Query(ctx, `SELECT COALESCE(action,event_type),COALESCE(description,''),COALESCE(u.name,''),created_at FROM admin_security_activity a LEFT JOIN admin_users u ON u.id=a.admin_user_id ORDER BY a.created_at DESC LIMIT 30`)
	if err == nil {
		for adminRows.Next() {
			var action, description, actor string
			var at time.Time
			if adminRows.Scan(&action, &description, &actor, &at) == nil {
				activity = append(activity, dashboardActivity{Event: description, EventType: action, User: actor, Timestamp: at})
			}
		}
		adminRows.Close()
	}
	sort.Slice(activity, func(i, j int) bool { return activity[i].Timestamp.After(activity[j].Timestamp) })
	if len(activity) > 15 {
		activity = activity[:15]
	}
	revenue := map[string]map[string]float64{"monthly": {}, "annual": {}}
	revenueRows, revenueErr := h.DB.Query(ctx, `SELECT currency,COALESCE(SUM(CASE WHEN billing_period='monthly' THEN amount WHEN billing_period='yearly' THEN amount/12 ELSE 0 END),0),COALESCE(SUM(CASE WHEN billing_period='monthly' THEN amount*12 WHEN billing_period='yearly' THEN amount ELSE 0 END),0) FROM subscriptions WHERE status='active' GROUP BY currency`)
	if revenueErr == nil {
		for revenueRows.Next() {
			var currency string
			var monthly, annual float64
			if revenueRows.Scan(&currency, &monthly, &annual) == nil {
				revenue["monthly"][currency] = monthly
				revenue["annual"][currency] = annual
			}
		}
		revenueRows.Close()
	}
	writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"metrics": m, "revenue": revenue, "timeseries": series, "top_cards": top, "recent_activity": activity}})
}

func activityLabel(t string) string {
	switch t {
	case "user_registered":
		return "User registered"
	case "organization_created":
		return "Organization created"
	case "card_created":
		return "Digital card created"
	case "card_published":
		return "Digital card published"
	case "qr_generated":
		return "QR code generated"
	case "qr_scan":
		return "QR code scanned"
	case "nfc_assigned":
		return "NFC tag assigned"
	case "nfc_tap":
		return "NFC tag tapped"
	case "contact_exchange":
		return "Contact saved"
	case "subscription_activated":
		return "Subscription activated"
	default:
		return t
	}
}
