package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// OTPService is shared by customer and administrator auth while keeping their
// subjects, destinations and password tables separate.
type OTPService struct {
	DB     *pgxpool.Pool
	Sender EmailSender
}

var ErrOTPThrottle = errors.New("OTP request limit reached")

func (s *OTPService) Issue(subjectType string, subjectID int64, destination, purpose string) error {
	if s.Sender == nil {
		if _, err := smtpConfigFromEnv(); err != nil {
			return errors.New("OTP email delivery is not configured")
		}
	}
	tx, err := s.DB.Begin(context.Background())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	lockKey := subjectType + ":" + destination + ":" + purpose
	if _, err = tx.Exec(context.Background(), `SELECT pg_advisory_xact_lock(hashtext($1))`, lockKey); err != nil {
		return err
	}
	var count int
	if err = tx.QueryRow(context.Background(), `SELECT COUNT(*) FROM auth_otp_codes WHERE subject_type=$1 AND destination=$2 AND purpose=$3 AND created_at>NOW()-INTERVAL '1 hour'`, subjectType, destination, purpose).Scan(&count); err != nil {
		return err
	}
	if count >= 5 {
		return ErrOTPThrottle
	}
	var last time.Time
	err = tx.QueryRow(context.Background(), `SELECT created_at FROM auth_otp_codes WHERE subject_type=$1 AND destination=$2 AND purpose=$3 ORDER BY created_at DESC LIMIT 1`, subjectType, destination, purpose).Scan(&last)
	if err == nil && time.Since(last) < time.Minute {
		return ErrOTPThrottle
	}
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	var expiresAt time.Time
	max := big.NewInt(1000000)
	value, err := rand.Int(rand.Reader, max)
	if err != nil {
		return err
	}
	code := fmt.Sprintf("%06d", value.Int64())
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = tx.Exec(context.Background(), `UPDATE auth_otp_codes SET used_at=COALESCE(used_at,NOW()),verification_token_used_at=CASE WHEN verification_token_hash IS NOT NULL AND verification_token_used_at IS NULL THEN NOW() ELSE verification_token_used_at END WHERE subject_type=$1 AND destination=$2 AND purpose=$3 AND (used_at IS NULL OR (verification_token_hash IS NOT NULL AND verification_token_used_at IS NULL))`, subjectType, destination, purpose)
	if err != nil {
		return err
	}
	err = tx.QueryRow(context.Background(), `INSERT INTO auth_otp_codes(subject_type,subject_id,destination,purpose,code_hash,expires_at) VALUES($1,$2,$3,$4,$5,NOW()+INTERVAL '10 minutes') RETURNING expires_at`, subjectType, subjectID, destination, purpose, string(hash)).Scan(&expiresAt)
	if err != nil {
		return err
	}
	sender := s.Sender
	if sender == nil {
		sender = ZeptoMailSMTP{}
	}
	subject, _, _ := otpEmailContent(purpose, code, expiresAt)
	sendCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err = sender.SendOTP(sendCtx, destination, subject, code, purpose, expiresAt); err != nil {
		return errors.New("OTP email could not be delivered")
	}
	if err = tx.Commit(context.Background()); err != nil {
		return err
	}
	return nil
}

// Verify consumes one valid code and returns its subject ID plus an opaque,
// one-use password-reset token (the token is empty for registration OTPs).
func (s *OTPService) Verify(subjectType, destination, purpose, code string) (int64, string, error) {
	tx, err := s.DB.Begin(context.Background())
	if err != nil {
		return 0, "", err
	}
	defer tx.Rollback(context.Background())
	var id, subjectID int64
	var hash string
	err = tx.QueryRow(context.Background(), `SELECT id,subject_id,code_hash FROM auth_otp_codes WHERE subject_type=$1 AND destination=$2 AND purpose=$3 AND used_at IS NULL AND expires_at>NOW() AND attempts<max_attempts ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, subjectType, destination, purpose).Scan(&id, &subjectID, &hash)
	if err != nil {
		return 0, "", fmt.Errorf("invalid or expired code")
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) != nil {
		_, _ = tx.Exec(context.Background(), `UPDATE auth_otp_codes SET attempts=attempts+1,used_at=CASE WHEN attempts+1>=max_attempts THEN NOW() ELSE used_at END WHERE id=$1`, id)
		_ = tx.Commit(context.Background())
		return 0, "", fmt.Errorf("invalid or expired code")
	}
	var resetToken string
	if purpose != "customer_registration" {
		buf := make([]byte, 32)
		if _, err = rand.Read(buf); err != nil {
			return 0, "", err
		}
		resetToken = hex.EncodeToString(buf)
		digest := sha256.Sum256([]byte(resetToken))
		_, err = tx.Exec(context.Background(), `UPDATE auth_otp_codes SET used_at=NOW(),verified_at=NOW(),verification_token_hash=$2 WHERE id=$1`, id, hex.EncodeToString(digest[:]))
	} else {
		_, err = tx.Exec(context.Background(), `UPDATE auth_otp_codes SET used_at=NOW(),verified_at=NOW() WHERE id=$1`, id)
	}
	if err != nil {
		return 0, "", err
	}
	if err = tx.Commit(context.Background()); err != nil {
		return 0, "", err
	}
	return subjectID, resetToken, nil
}

func resetTokenHash(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func (s *OTPService) ConsumeResetToken(subjectType, destination, purpose, token string) (int64, error) {
	if strings.TrimSpace(token) == "" {
		return 0, fmt.Errorf("invalid or expired reset verification")
	}
	var subjectID int64
	err := s.DB.QueryRow(context.Background(), `WITH chosen AS (SELECT id FROM auth_otp_codes WHERE subject_type=$1 AND destination=$2 AND purpose=$3 AND verification_token_hash=$4 AND verified_at IS NOT NULL AND verification_token_used_at IS NULL AND expires_at>NOW() ORDER BY created_at DESC LIMIT 1 FOR UPDATE SKIP LOCKED) UPDATE auth_otp_codes a SET verification_token_used_at=NOW() FROM chosen WHERE a.id=chosen.id AND a.verification_token_used_at IS NULL RETURNING a.subject_id`, subjectType, destination, purpose, resetTokenHash(token)).Scan(&subjectID)
	if err != nil {
		return 0, fmt.Errorf("invalid or expired reset verification")
	}
	return subjectID, nil
}
