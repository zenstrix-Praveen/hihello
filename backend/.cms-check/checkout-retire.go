// Retire only the disposable local QA identities; preserve all billing history.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"os"
	"strings"
	"time"
)

func main() {
	_ = godotenv.Load()
	if os.Getenv("APP_ENV") != "local" {
		panic("Local only")
	}
	cfg, e := pgx.ParseConfig(os.Getenv("DATABASE_URL"))
	if e != nil || (cfg.Host != "localhost" && cfg.Host != "127.0.0.1") {
		panic("Local DB only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, e := pgx.ConnectConfig(ctx, cfg)
	if e != nil {
		panic("Local DB unavailable")
	}
	defer db.Close(ctx)
	for _, file := range []string{"../web/.feature-qa/upgrade-customer.json", "../web/.feature-qa/trial-customer.json"} {
		raw, e := os.ReadFile(file)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			panic("QA identity missing")
		}
		var a struct {
			ID    int64  `json:"id"`
			Email string `json:"email"`
		}
		if json.Unmarshal(raw, &a) != nil || a.ID < 1 || !strings.HasPrefix(a.Email, "upgrade-qa-") || !strings.HasSuffix(a.Email, "@example.invalid") {
			panic("Invalid QA identity")
		}
		tag, e := db.Exec(ctx, `UPDATE users SET status='inactive',updated_at=NOW() WHERE id=$1 AND email=$2 AND name='Upgrade QA disposable'`, a.ID, a.Email)
		if e != nil || tag.RowsAffected() != 1 {
			panic("QA identity mismatch")
		}
		if os.Remove(file) != nil {
			panic("QA credential deletion failed")
		}
		fmt.Printf("Retired disposable QA customer %d; subscription and payment history preserved\n", a.ID)
	}
}
