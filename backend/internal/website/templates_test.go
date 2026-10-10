package website

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Set WEBSITE_TEST_DATABASE_URL to run these read-only endpoint regressions
// against a migrated PostgreSQL test database.
func TestTemplateEndpointsLoadFromDatabase(t *testing.T) {
	dsn := os.Getenv("WEBSITE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set WEBSITE_TEST_DATABASE_URL to run PostgreSQL endpoint regression tests")
	}
	db, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(db.Close)

	for name, handler := range map[string]http.HandlerFunc{
		"admin templates":  (&TemplateHandler{DB: db}).List,
		"public templates": (&PublicTemplateHandler{DB: db}).List,
	} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			handler(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("endpoint returned %d: %s", w.Code, w.Body.String())
			}
		})
	}
}
