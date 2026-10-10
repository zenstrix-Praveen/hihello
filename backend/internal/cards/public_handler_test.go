package cards

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type publicTestStore struct {
	published bool
	card      json.RawMessage
	events    int
	lastSQL   string
	queries   []string
}

func (s *publicTestStore) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	s.lastSQL = query
	s.queries = append(s.queries, query)
	if !s.published {
		return publicTestRow{err: pgx.ErrNoRows}
	}
	if strings.Contains(query, "jsonb_build_object") {
		return publicTestRow{values: []any{int64(42), int64(17), s.card}}
	}
	return publicTestRow{values: []any{int64(42), int64(17)}}
}
func (s *publicTestStore) Exec(_ context.Context, query string, _ ...any) (pgconn.CommandTag, error) {
	s.lastSQL = query
	s.events++
	return pgconn.CommandTag{}, nil
}

type publicTestRow struct {
	values []any
	err    error
}

func (r publicTestRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, value := range r.values {
		reflect.ValueOf(dest[i]).Elem().Set(reflect.ValueOf(value))
	}
	return nil
}

func publicTestCard() json.RawMessage {
	return json.RawMessage(`{"id":42,"public_id":"public-uuid","name":"Jordan Example","slug":"jordan-example","job_title":"Designer","company_name":"Studio","phone":"+14155550100","email":"jordan@example.com","website":"https://example.com","address":"1 Main Street","photo_url":"","logo_url":"","primary_color":"#1267E8","template_id":2,"template_name":"Blue","social_links":[],"custom_links":[],"videos":[],"published_at":"2026-01-01T00:00:00Z"}`)
}

func publicTestRouter(handler *PublicHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/public/cards/{slug}", handler.Get)
	r.Get("/public/cards/{slug}/vcard", handler.VCard)
	r.Post("/public/cards/{slug}/events", handler.Event)
	return r
}

func TestPublicCardOnlyServesPublishedProjection(t *testing.T) {
	store := &publicTestStore{published: true, card: publicTestCard()}
	handler := &PublicHandler{DB: store}
	req := httptest.NewRequest(http.MethodGet, "/public/cards/jordan-example", nil)
	res := httptest.NewRecorder()
	publicTestRouter(handler).ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", res.Code, res.Body.String())
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(result["data"], &data); err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"user_id", "organization_id", "owner_name", "owner_email", "password_hash", "role_id"} {
		if _, ok := data[private]; ok {
			t.Errorf("public response contains private field %q", private)
		}
	}
	if len(store.queries) == 0 || !strings.Contains(store.queries[0], "c.status='published'") {
		t.Fatal("public query does not enforce published status")
	}
	if store.events != 1 {
		t.Fatalf("card view events = %d, want 1", store.events)
	}
}

func TestUnpublishedAndUnknownPublicCardsReturnSameNotFound(t *testing.T) {
	for _, slug := range []string{"draft-card", "suspended-card", "unknown-card"} {
		store := &publicTestStore{published: false, card: publicTestCard()}
		res := httptest.NewRecorder()
		publicTestRouter(&PublicHandler{DB: store}).ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/public/cards/"+slug, nil))
		if res.Code != http.StatusNotFound {
			t.Errorf("slug %q status = %d, want 404", slug, res.Code)
		}
		if store.events != 0 {
			t.Errorf("slug %q unexpectedly recorded analytics", slug)
		}
	}
}

func TestPublicVCardDownload(t *testing.T) {
	store := &publicTestStore{published: true, card: publicTestCard()}
	res := httptest.NewRecorder()
	publicTestRouter(&PublicHandler{DB: store}).ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/public/cards/jordan-example/vcard", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if got := res.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/vcard") {
		t.Fatalf("content type = %q", got)
	}
	for _, field := range []string{"BEGIN:VCARD", "FN:Jordan Example", "ORG:Studio", "TITLE:Designer", "TEL;TYPE=CELL:+14155550100", "EMAIL;TYPE=INTERNET:jordan@example.com", "URL:https://example.com", "ADR;TYPE=WORK:;;1 Main Street;;;;", "END:VCARD"} {
		if !strings.Contains(res.Body.String(), field) {
			t.Errorf("vCard missing %q", field)
		}
	}
	if store.events != 0 {
		t.Fatal("vCard response should not wait for an analytics write")
	}
}

func TestPublicEventsOnlyAcceptKnownPublishedActions(t *testing.T) {
	for _, tc := range []struct {
		body       string
		status     int
		wantEvents int
	}{
		{`{"type":"link_click","action":"call"}`, http.StatusNoContent, 1},
		{`{"type":"contact_exchange","action":"save_contact"}`, http.StatusNoContent, 1},
		{`{"type":"card_view","action":"arbitrary"}`, http.StatusBadRequest, 0},
		{`{"type":"link_click","action":"https://other.invalid"}`, http.StatusBadRequest, 0},
	} {
		store := &publicTestStore{published: true}
		res := httptest.NewRecorder()
		publicTestRouter(&PublicHandler{DB: store}).ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/public/cards/jordan-example/events", strings.NewReader(tc.body)))
		if res.Code != tc.status {
			t.Errorf("body %s status = %d, want %d", tc.body, res.Code, tc.status)
		}
		if store.events != tc.wantEvents {
			t.Errorf("body %s events = %d, want %d", tc.body, store.events, tc.wantEvents)
		}
	}
}

func TestVCardEscapesSpecialCharacters(t *testing.T) {
	got := vCardEscape("A, B; C\\D\nE")
	if got != `A\, B\; C\\D\nE` {
		t.Fatalf("escape = %q", got)
	}
}
