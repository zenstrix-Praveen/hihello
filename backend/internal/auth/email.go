package auth

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

type smtpConfig struct {
	host     string
	port     int
	username string
	password string
	from     string
	fromName string
}

type EmailSender interface {
	SendOTP(ctx context.Context, to, subject, code, purpose string, expiresAt time.Time) error
}

// ZeptoMailSMTP sends OTP email using server-side SMTP configuration.
type ZeptoMailSMTP struct{}

type securePlainAuth struct {
	username    string
	password    string
	implicitTLS bool
}

func (a securePlainAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS && !a.implicitTLS {
		return "", nil, errors.New("SMTP authentication requires TLS")
	}
	return "PLAIN", []byte("\x00" + a.username + "\x00" + a.password), nil
}

func (securePlainAuth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return nil, errors.New("unexpected SMTP authentication challenge")
	}
	return nil, nil
}

func smtpConfigFromEnv() (smtpConfig, error) {
	cfg := smtpConfig{
		host:     strings.TrimSpace(firstEnv("SMTP_HOST", "MAIL_HOST")),
		username: firstEnv("SMTP_USERNAME", "MAIL_USERNAME"),
		password: firstEnv("SMTP_PASSWORD", "MAIL_PASSWORD"),
		fromName: strings.TrimSpace(firstEnv("SMTP_FROM_NAME", "MAIL_FROM_NAME")),
	}
	port, err := strconv.Atoi(strings.TrimSpace(firstEnv("SMTP_PORT", "MAIL_PORT")))
	if err != nil || port < 1 || port > 65535 {
		return smtpConfig{}, errors.New("invalid SMTP port")
	}
	cfg.port = port
	from, err := mail.ParseAddress(strings.TrimSpace(firstEnv("SMTP_FROM_EMAIL", "MAIL_FROM_EMAIL", "MAIL_FROM_ADDRESS")))
	if err != nil || from.Address == "" || strings.ContainsAny(cfg.host, "\r\n") || cfg.host == "" || cfg.fromName == "" || strings.ContainsAny(cfg.fromName, "\r\n") {
		return smtpConfig{}, errors.New("invalid SMTP configuration")
	}
	if (cfg.username == "") != (cfg.password == "") || cfg.username == "" {
		return smtpConfig{}, errors.New("SMTP username and password are required")
	}
	cfg.from = from.Address
	return cfg, nil
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := os.Getenv(name); strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (s *OTPService) EmailDeliveryConfigured() bool {
	_, err := smtpConfigFromEnv()
	return err == nil
}

// ValidateProductionSMTPConfig ensures OTP email delivery has non-placeholder
// credentials before the production server starts. Connectivity is checked at
// send time so a temporary SMTP outage does not prevent unrelated recovery.
func ValidateProductionSMTPConfig() error {
	cfg, err := smtpConfigFromEnv()
	if err != nil {
		return errors.New("SMTP configuration is required in production")
	}
	for _, item := range []string{cfg.host, cfg.username, cfg.password, cfg.from, cfg.fromName} {
		value := strings.ToLower(strings.TrimSpace(item))
		if strings.Contains(value, "example.com") || strings.Contains(value, ".example") || strings.Contains(value, "replace") || strings.Contains(value, "placeholder") || strings.Contains(value, "changeme") || strings.Contains(value, "change-me") || strings.Contains(value, "your_") {
			return errors.New("SMTP configuration must not contain placeholder values")
		}
	}
	return nil
}

func otpEmailContent(purpose, code string, expiresAt time.Time) (string, string, error) {
	subject := ""
	switch purpose {
	case "customer_registration":
		subject = "Verify your Dbizcard account"
	case "customer_password_reset":
		subject = "Reset your Dbizcard password"
	case "admin_password_reset":
		subject = "Dbizcard administrator password reset"
	default:
		return "", "", errors.New("unsupported OTP email purpose")
	}
	minutes := int(time.Until(expiresAt).Minutes() + 0.99)
	if minutes < 1 {
		minutes = 1
	}
	body := fmt.Sprintf("Hello,\n\nYour verification code is:\n\n%s\n\nThis code expires in %d minutes.\n\nDo not share this code with anyone.\n\nDbizcard\n", code, minutes)
	return subject, body, nil
}

// SendOTP delivers a generated OTP without exposing SMTP details or the
// code through logs or API responses.
func (ZeptoMailSMTP) SendOTP(ctx context.Context, to, subject, code, purpose string, expiresAt time.Time) error {
	expectedSubject, body, err := otpEmailContent(purpose, code, expiresAt)
	if err != nil {
		return err
	}
	// Purpose remains authoritative so an arbitrary caller cannot inject a
	// different subject into this transactional email.
	if subject != expectedSubject {
		return errors.New("invalid OTP email subject")
	}
	return (ZeptoMailSMTP{}).SendEmail(ctx, to, subject, body)
}

// SendEmail delivers a plain-text transactional email using the same server-side SMTP transport as OTP delivery.
func (ZeptoMailSMTP) SendEmail(ctx context.Context, to, subject, body string) error {
	cfg, err := smtpConfigFromEnv()
	if err != nil {
		return err
	}
	recipient, err := mail.ParseAddress(to)
	if err != nil || recipient.Address != to || strings.ContainsAny(to, "\r\n") {
		return errors.New("invalid email recipient")
	}
	if subject == "" || strings.ContainsAny(subject, "\r\n") || strings.TrimSpace(body) == "" {
		return errors.New("invalid email message")
	}
	from := (&mail.Address{Name: cfg.fromName, Address: cfg.from}).String()
	message := strings.Join([]string{
		"From: " + from,
		"To: " + recipient.String(),
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		body,
	}, "\r\n")
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	address := net.JoinHostPort(cfg.host, strconv.Itoa(cfg.port))
	tlsConfig := &tls.Config{ServerName: cfg.host, MinVersion: tls.VersionTLS12}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return errors.New("could not connect to SMTP server")
	}
	if cfg.port == 465 {
		tlsConn := tls.Client(conn, tlsConfig)
		if err = tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return errors.New("SMTP TLS negotiation failed")
		}
		conn = tlsConn
	}
	defer conn.Close()
	deadline := time.Now().Add(15 * time.Second)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	_ = conn.SetDeadline(deadline)
	client, err := smtp.NewClient(conn, cfg.host)
	if err != nil {
		return errors.New("could not initialize SMTP connection")
	}
	defer client.Close()
	if cfg.port != 465 {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("SMTP server does not support STARTTLS")
		}
		if err = client.StartTLS(tlsConfig); err != nil {
			return errors.New("SMTP STARTTLS negotiation failed")
		}
	}
	if err = client.Auth(securePlainAuth{username: cfg.username, password: cfg.password, implicitTLS: cfg.port == 465}); err != nil {
		return errors.New("SMTP authentication failed")
	}
	if err = client.Mail(cfg.from); err != nil {
		return errors.New("SMTP sender was rejected")
	}
	if err = client.Rcpt(recipient.Address); err != nil {
		return errors.New("SMTP recipient was rejected")
	}
	writer, err := client.Data()
	if err != nil {
		return errors.New("SMTP message was rejected")
	}
	if _, err = io.WriteString(writer, message); err != nil {
		_ = writer.Close()
		return errors.New("could not write OTP email")
	}
	if err = writer.Close(); err != nil {
		return errors.New("SMTP did not accept OTP email")
	}
	_ = client.Quit()
	return nil
}
