package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"digital-card-platform/backend/internal/database"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found; reading environment variables")
	}

	userIDText := strings.TrimSpace(os.Getenv("USER_SEED_ID"))
	password := os.Getenv("USER_SEED_PASSWORD")

	if userIDText == "" {
		log.Fatal("USER_SEED_ID is required")
	}

	if password == "" {
		log.Fatal("USER_SEED_PASSWORD is required")
	}

	if len(password) < 8 {
		log.Fatal("USER_SEED_PASSWORD must be at least 8 characters")
	}

	userID, err := strconv.ParseInt(userIDText, 10, 64)
	if err != nil || userID < 1 {
		log.Fatal("USER_SEED_ID must be a valid positive integer")
	}

	db, err := database.ConnectPostgres()
	if err != nil {
		log.Fatalf("PostgreSQL connection failed: %v", err)
	}
	defer db.Close()

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		log.Fatalf("Password hashing failed: %v", err)
	}

	tag, err := db.Exec(
		context.Background(),
		`
        UPDATE users
        SET
            password_hash = $1,
            status = 'active',
            updated_at = NOW()
        WHERE id = $2
        `,
		string(hash),
		userID,
	)
	if err != nil {
		log.Fatalf("Failed to update user password: %v", err)
	}

	if tag.RowsAffected() == 0 {
		log.Fatalf("No user found with id %d", userID)
	}

	fmt.Println("User password saved successfully")
	fmt.Println("User ID:", userID)
	fmt.Println("Password is stored only as a bcrypt hash")
}
