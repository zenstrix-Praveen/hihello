// Disposable local customer for real browser login and unsupported-currency QA.
// No payment is generated or activated by this helper.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"os"
	"strings"
	"time"
)

type account struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

var filename = "../web/.feature-qa/upgrade-customer.json"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "trial" {
		filename = "../web/.feature-qa/trial-customer.json"
	}
	_ = godotenv.Load()
	if os.Getenv("APP_ENV") != "local" {
		panic("Disposable QA customers require APP_ENV=local")
	}
	cfg, err := pgx.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		panic("Invalid local DB configuration")
	}
	if cfg.Host != "localhost" && cfg.Host != "127.0.0.1" && cfg.Host != "::1" {
		panic("Disposable customer QA is restricted to a localhost database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		panic("Local QA database unavailable")
	}
	defer db.Close(ctx)
	if len(os.Args) > 1 && os.Args[1] == "cleanup" {
		raw, err := os.ReadFile(filename)
		if err != nil {
			panic("QA account file unavailable")
		}
		var a account
		if json.Unmarshal(raw, &a) != nil || a.ID < 1 || !strings.HasPrefix(a.Email, "upgrade-qa-") || !strings.HasSuffix(a.Email, "@example.invalid") {
			panic("Refusing to clean up an unrecognized account")
		}
		tx, err := db.Begin(ctx)
		if err != nil {
			panic("QA cleanup could not begin")
		}
		defer tx.Rollback(ctx)
		var payments int
		if tx.QueryRow(ctx, `SELECT COUNT(*) FROM payments WHERE user_id=$1`, a.ID).Scan(&payments) != nil || payments != 0 {
			panic("QA cleanup refused: preserve payment history")
		}
		var verifiedID int64
		if tx.QueryRow(ctx, `SELECT id FROM users WHERE id=$1 AND email=$2 AND name='Upgrade QA disposable' FOR UPDATE`, a.ID, a.Email).Scan(&verifiedID) != nil {
			panic("QA customer identity mismatch")
		}
		if _, err := tx.Exec(ctx, `DELETE FROM subscriptions WHERE user_id=$1`, verifiedID); err != nil {
			panic("QA subscription cleanup failed")
		}
		if _, err := tx.Exec(ctx, `DELETE FROM users WHERE id=$1 AND email=$2`, verifiedID, a.Email); err != nil {
			panic("QA customer cleanup failed")
		}
		if tx.Commit(ctx) != nil {
			panic("QA cleanup commit failed")
		}
		if os.Remove(filename) != nil {
			panic("QA credential cleanup failed")
		}
		fmt.Println("Removed disposable QA customer and its free-only subscription; no payments existed")
		return
	}
	if _, err := os.Stat(filename); err == nil {
		panic("Existing QA account file must be cleaned up first")
	}
	entropy := make([]byte, 24)
	if _, err := rand.Read(entropy); err != nil {
		panic("QA entropy unavailable")
	}
	a := account{Email: "upgrade-qa-" + hex.EncodeToString(entropy[:8]) + "@example.invalid", Password: hex.EncodeToString(entropy)}
	hash, err := bcrypt.GenerateFromPassword([]byte(a.Password), bcrypt.DefaultCost)
	if err != nil {
		panic("QA password hash failed")
	}
	if err := db.QueryRow(ctx, `INSERT INTO users(name,email,password_hash,status,email_verified_at) VALUES('Upgrade QA disposable',$1,$2,'active',NOW()) RETURNING id`, a.Email, string(hash)).Scan(&a.ID); err != nil {
		panic("QA customer creation failed")
	}
	raw, _ := json.Marshal(a)
	if err := os.WriteFile(filename, raw, 0600); err != nil {
		panic("QA account file could not be written")
	}
	fmt.Println("Created a disposable localhost QA customer; credentials stored only in .feature-qa")
}
