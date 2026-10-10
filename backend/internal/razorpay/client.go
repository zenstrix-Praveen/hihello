package razorpay

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	appcurrency "digital-card-platform/backend/internal/currency"
)

const apiBase = "https://api.razorpay.com/v1"

type Client struct {
	KeyID     string
	KeySecret string
	HTTP      *http.Client
}

type Order struct {
	ID       string `json:"id"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Receipt  string `json:"receipt"`
	Status   string `json:"status"`
}

type Payment struct {
	ID               string `json:"id"`
	OrderID          string `json:"order_id"`
	Amount           int64  `json:"amount"`
	Currency         string `json:"currency"`
	Status           string `json:"status"`
	Method           string `json:"method"`
	Bank             string `json:"bank"`
	Wallet           string `json:"wallet"`
	ErrorCode        string `json:"error_code"`
	ErrorDescription string `json:"error_description"`
}

type apiError struct {
	Error struct {
		Code        string `json:"code"`
		Description string `json:"description"`
	} `json:"error"`
}

// ProviderError contains only bounded, credential-redacted diagnostics.
type ProviderError struct {
	Kind              string
	Status            int
	Code, Description string
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("Razorpay kind=%s HTTP=%d code=%s description=%s", e.Kind, e.Status, e.Code, e.Description)
}
func FailureKind(err error) string {
	var failure *ProviderError
	if errors.As(err, &failure) {
		return failure.Kind
	}
	return "unknown"
}

func (c Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (c Client) request(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, apiBase+path, reader)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.KeyID, c.KeySecret)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		kind := "network"
		var timed interface{ Timeout() bool }
		if errors.As(err, &timed) && timed.Timeout() {
			kind = "timeout"
		}
		return &ProviderError{Kind: kind}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return errors.New("could not read Razorpay response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr apiError
		_ = json.Unmarshal(data, &apiErr)
		// Only bounded, redacted provider diagnostics; never headers or raw response bodies.
		clean := func(value string) string {
			for _, secret := range []string{c.KeySecret, c.KeyID} {
				if secret != "" {
					value = strings.ReplaceAll(value, secret, "[redacted]")
				}
			}
			value = strings.Map(func(r rune) rune {
				if r < ' ' {
					return ' '
				}
				return r
			}, value)
			if len(value) > 300 {
				value = value[:300]
			}
			return value
		}
		kind := "request"
		description := clean(apiErr.Error.Description)
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			kind = "authentication"
		} else if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			kind = "unavailable"
		} else if strings.Contains(strings.ToLower(description), "currency") || strings.Contains(strings.ToLower(description), "international") {
			kind = "currency"
		}
		return &ProviderError{Kind: kind, Status: resp.StatusCode, Code: clean(apiErr.Error.Code), Description: description}
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return errors.New("invalid Razorpay response")
		}
	}
	return nil
}

func (c Client) CreateOrder(ctx context.Context, amount int64, currency, receipt string) (Order, error) {
	var out Order
	err := c.request(ctx, http.MethodPost, "/orders", map[string]any{"amount": amount, "currency": currency, "receipt": receipt, "payment_capture": 1}, &out)
	return out, err
}

func (c Client) FetchOrder(ctx context.Context, orderID string) (Order, error) {
	var out Order
	err := c.request(ctx, http.MethodGet, "/orders/"+orderID, nil, &out)
	return out, err
}

func (c Client) FetchPayment(ctx context.Context, paymentID string) (Payment, error) {
	var out Payment
	err := c.request(ctx, http.MethodGet, "/payments/"+paymentID, nil, &out)
	return out, err
}

func VerifyCheckoutSignature(orderID, paymentID, signature, secret string) bool {
	if orderID == "" || paymentID == "" || signature == "" || secret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(orderID + "|" + paymentID))
	expected := mac.Sum(nil)
	provided, err := hex.DecodeString(signature)
	return err == nil && hmac.Equal(expected, provided)
}

func VerifyWebhookSignature(rawBody []byte, signature, secret string) bool {
	if len(rawBody) == 0 || signature == "" || secret == "" {
		return false
	}
	provided, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(rawBody)
	return hmac.Equal(mac.Sum(nil), provided)
}

var decimalAmount = regexp.MustCompile(`^(0|[1-9][0-9]*)(?:\.([0-9]+))?$`)

func CurrencyExponent(currency string) int {
	if exponent, ok := appcurrency.Exponent(currency); ok {
		return exponent
	}
	switch strings.ToUpper(currency) {
	case "BIF", "CLP", "DJF", "GNF", "ISK", "JPY", "KMF", "KRW", "PYG", "RWF", "UGX", "VND", "VUV", "XAF", "XOF", "XPF":
		return 0
	case "BHD", "IQD", "JOD", "KWD", "LYD", "OMR", "TND":
		return 3
	default:
		return 2
	}
}

// AmountToMinor converts a decimal major-unit amount without float rounding.
func AmountToMinor(amount, currency string) (int64, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if !appcurrency.Supported(currency) {
		return 0, errors.New("invalid currency")
	}
	matches := decimalAmount.FindStringSubmatch(strings.TrimSpace(amount))
	if matches == nil {
		return 0, errors.New("invalid amount")
	}
	exponent := CurrencyExponent(currency)
	fraction := matches[2]
	if len(fraction) > exponent && strings.Trim(fraction[exponent:], "0") != "" {
		return 0, errors.New("amount has unsupported fractional precision")
	}
	if len(fraction) > exponent {
		fraction = fraction[:exponent]
	}
	fraction += strings.Repeat("0", exponent-len(fraction))
	digits := strings.TrimLeft(matches[1]+fraction, "0")
	if digits == "" {
		digits = "0"
	}
	n, ok := new(big.Int).SetString(digits, 10)
	if !ok || !n.IsInt64() {
		return 0, errors.New("amount exceeds supported range")
	}
	return n.Int64(), nil
}

// Razorpay requires the final subunit digit to be zero for three-decimal
// currencies. Reject instead of rounding or changing the configured price.
func CheckoutAmountToMinor(amount, currency string) (int64, error) {
	minor, err := AmountToMinor(amount, currency)
	if err != nil {
		return 0, err
	}
	if strings.EqualFold(strings.TrimSpace(currency), "INR") && minor < 100 {
		return 0, errors.New("Razorpay INR minimum order amount is 100 paise")
	}
	if CurrencyExponent(currency) == 3 && minor%10 != 0 {
		return 0, errors.New("gateway cannot charge this exact three-decimal price")
	}
	return minor, nil
}
