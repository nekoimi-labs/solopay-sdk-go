package solopay

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nekoimi/solopay-sdk-go/internal/signing"
	"github.com/nekoimi/solopay-sdk-go/internal/wire"
)

const (
	headerAppID          = "X-App-Id"
	headerTimestamp      = "X-Timestamp"
	headerNonce          = "X-Nonce"
	headerSignature      = "X-Signature"
	headerRequestID      = "X-Request-ID"
	headerIdempotencyKey = "Idempotency-Key"

	maxResponseBytes = 2 << 20
)

// Client calls SoloPay's merchant-facing payment API. It is safe for
// concurrent use after New returns.
type Client struct {
	baseURL   *url.URL
	appID     string
	appSecret []byte
	http      *http.Client
	userAgent string
	location  *time.Location
	now       func() time.Time
	nonce     func() (string, error)
}

// New creates a payment client.
func New(cfg Config) (*Client, error) {
	base, cfg, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL:   base,
		appID:     cfg.AppID,
		appSecret: []byte(cfg.AppSecret),
		http:      cfg.HTTPClient,
		userAgent: cfg.UserAgent,
		location:  cfg.ServerLocation,
		now:       time.Now,
		nonce:     randomNonce,
	}, nil
}

// NewClient is an alias for New.
func NewClient(cfg Config) (*Client, error) { return New(cfg) }

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body []byte, idempotencyKey string, out any) error {
	u := *c.baseURL
	u.Path = path
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("solopay: build request: %w", err)
	}
	timestamp := c.now().Unix()
	nonce, err := c.nonce()
	if err != nil {
		return fmt.Errorf("solopay: create nonce: %w", err)
	}
	// SoloPay verifies inbound API signatures against net/url's decoded Path.
	canonical := signing.Canonical(method, req.URL.Path, timestamp, nonce, body)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set(headerAppID, c.appID)
	req.Header.Set(headerTimestamp, strconv.FormatInt(timestamp, 10))
	req.Header.Set(headerNonce, nonce)
	req.Header.Set(headerSignature, signing.SignHex(c.appSecret, canonical))
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		req.Header.Set(headerIdempotencyKey, idempotencyKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("solopay: perform request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("solopay: read response: %w", err)
	}
	if len(raw) > maxResponseBytes {
		return &DecodeError{HTTPStatus: resp.StatusCode, Err: fmt.Errorf("response exceeds %d bytes", maxResponseBytes)}
	}

	var envelope wire.Envelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return &APIError{
				HTTPStatus: resp.StatusCode,
				Message:    strings.TrimSpace(http.StatusText(resp.StatusCode)),
				RequestID:  resp.Header.Get(headerRequestID),
			}
		}
		return &DecodeError{HTTPStatus: resp.StatusCode, Err: err}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || envelope.Code != 0 {
		message := strings.TrimSpace(envelope.Message)
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return &APIError{
			HTTPStatus: resp.StatusCode,
			Code:       envelope.Code,
			Message:    message,
			Detail:     cloneRawMessage(envelope.Error),
			RequestID:  resp.Header.Get(headerRequestID),
		}
	}
	if out == nil {
		return nil
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return &DecodeError{HTTPStatus: resp.StatusCode, Err: fmt.Errorf("successful response has no data")}
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return &DecodeError{HTTPStatus: resp.StatusCode, Err: err}
	}
	return nil
}

func randomNonce() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}
