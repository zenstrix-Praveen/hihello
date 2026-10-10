package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"digital-card-platform/backend/internal/database"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found; using system environment variables")
	}

	name := strings.TrimSpace(os.Getenv("ADMIN_SEED_NAME"))
	email := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_SEED_EMAIL")))
	password := os.Getenv("ADMIN_SEED_PASSWORD")

	if name == "" {
		log.Fatal("ADMIN_SEED_NAME is required")
	}

	if email == "" {
		log.Fatal("ADMIN_SEED_EMAIL is required")
	}

	if password == "" {
		log.Fatal("ADMIN_SEED_PASSWORD is required")
	}

	if len(password) < 12 {
		log.Fatal("ADMIN_SEED_PASSWORD must be at least 12 characters")
	}

	db, err := database.ConnectPostgres()
	if err != nil {
		log.Fatalf("PostgreSQL connection failed: %v", err)
	}
	defer db.Close()

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		log.Fatalf("Password hashing failed: %v", err)
	}

	_, err = db.Exec(
		context.Background(),
		`
		INSERT INTO admin_users (
			name,
			email,
			password_hash,
			role,
			status
		)
		VALUES (
			$1,
			$2,
			$3,
			'super_admin',
			'active'
		)
		ON CONFLICT (email)
		DO UPDATE SET
			name = EXCLUDED.name,
			password_hash = EXCLUDED.password_hash,
			role = 'super_admin',
			status = 'active',
			updated_at = NOW()
		`,
		name,
		email,
		string(passwordHash),
	)

	if err != nil {
		log.Fatalf("Failed to save admin: %v", err)
	}

	fmt.Println("Super admin saved successfully in PostgreSQL")
	fmt.Println("Email:", email)
	fmt.Println("Password is stored only as a bcrypt hash")
}
