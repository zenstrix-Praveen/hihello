package website

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"net/http"
	"strings"
	"testing"
)

type pricingContractDB struct {
	featureContractDB
	sql string
}

func (db *pricingContractDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	db.sql = sql
	return db.featureContractDB.Query(ctx, sql, args...)
}
func (db *pricingContractDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	db.sql = sql
	return db.featureContractDB.Exec(ctx, sql, args...)
}
func TestWebsitePricingArchiveContract(t *testing.T) {
	for _, tc := range []struct {
		name, method, body string
		handler            func(*ContentHandler) http.HandlerFunc
	}{
		{"admin list", "GET", "", func(h *ContentHandler) http.HandlerFunc { return h.ListPricing }},
		{"public list", "GET", "", func(h *ContentHandler) http.HandlerFunc { return h.PublicPricing }},
		{"publish", "PATCH", `{"is_enabled":true}`, func(h *ContentHandler) http.HandlerFunc { return h.SetPricingStatus }},
		{"edit", "PUT", `{"name":"Saved plan","currency":"USD","price":0}`, func(h *ContentHandler) http.HandlerFunc { return h.SavePricing }},
		{"delete", "DELETE", "", func(h *ContentHandler) http.HandlerFunc { return h.DeletePricing }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &pricingContractDB{featureContractDB: featureContractDB{affected: 1}}
			status, _ := featureContractRequest(t, tc.handler(&ContentHandler{DB: db}), tc.method, tc.body)
			if status != 200 {
				t.Fatalf("status %d", status)
			}
			if !strings.Contains(db.sql, "archived_at IS NULL") {
				t.Fatalf("archived records must not be listed or mutated: %s", db.sql)
			}
			if tc.name == "delete" && (!strings.Contains(db.sql, "archived_at=NOW()") || !strings.Contains(db.sql, "is_enabled=false")) {
				t.Fatalf("delete must preserve plan references and hide the plan: %s", db.sql)
			}
		})
	}
}
