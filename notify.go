package solopay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nekoimi-labs/solopay-sdk-go/internal/signing"
	"github.com/nekoimi-labs/solopay-sdk-go/internal/wire"
)

var (
	ErrNotifyMissingHeader    = errors.New("solopay: notification is missing signature headers")
	ErrNotifyInvalidSignature = errors.New("solopay: invalid notification signature")
	ErrNotifyInvalidTimestamp = errors.New("solopay: invalid notification timestamp")
	ErrNotifyAppIDMismatch    = errors.New("solopay: notification app ID mismatch")
	ErrNotifyReplay           = errors.New("solopay: notification nonce replay")
)

// PaymentNotification is the verified payment result sent to a merchant's
// notify URL.
type PaymentNotification struct {
	Event               string
	PlatformTradeNo     string
	OutTradeNo          string
	AppID               string
	Status              PaymentStatus
	AmountCents         int64
	OriginalAmountCents int64
	DiscountCents       int64
	MatchMode           MatchMode
	Currency            string
	Channel             Channel
	Subject             string
	PaidAt              *time.Time
	NotifyExtra         json.RawMessage
}

// ReplayStore can provide cross-process callback replay protection. Reserve
// returns true only when key was not already reserved in the TTL window.
type ReplayStore interface {
	Reserve(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// NotifyVerifierConfig configures callback verification.
type NotifyVerifierConfig struct {
	Secret        string
	ExpectedAppID string
	MaxClockSkew  time.Duration
	ReplayStore   ReplayStore

	// SignedPath overrides the request URL path when a reverse proxy rewrites
	// it before the application receives the callback.
	SignedPath string
}

// NotifyVerifier verifies signed payment callbacks. It is safe for concurrent
// use when its ReplayStore is safe for concurrent use.
type NotifyVerifier struct {
	secret        []byte
	expectedAppID string
	maxClockSkew  time.Duration
	replayStore   ReplayStore
	signedPath    string
	now           func() time.Time
}

func NewNotifyVerifier(cfg NotifyVerifierConfig) (*NotifyVerifier, error) {
	if cfg.Secret == "" {
		return nil, &ValidationError{Field: "Secret", Message: "is required"}
	}
	if cfg.MaxClockSkew <= 0 {
		cfg.MaxClockSkew = 5 * time.Minute
	}
	if cfg.SignedPath != "" && !strings.HasPrefix(cfg.SignedPath, "/") {
		return nil, &ValidationError{Field: "SignedPath", Message: "must start with /"}
	}
	return &NotifyVerifier{
		secret:        []byte(cfg.Secret),
		expectedAppID: strings.TrimSpace(cfg.ExpectedAppID),
		maxClockSkew:  cfg.MaxClockSkew,
		replayStore:   cfg.ReplayStore,
		signedPath:    cfg.SignedPath,
		now:           time.Now,
	}, nil
}

// VerifyNotification verifies and parses a callback with default settings.
// For expected-app checks, replay protection or proxy path overrides, create a
// reusable NotifyVerifier instead.
func VerifyNotification(req *http.Request, rawBody []byte, secret string) (*PaymentNotification, error) {
	verifier, err := NewNotifyVerifier(NotifyVerifierConfig{Secret: secret})
	if err != nil {
		return nil, err
	}
	return verifier.Parse(req, rawBody)
}

// Verify validates a callback signature without parsing its JSON body.
func (v *NotifyVerifier) Verify(req *http.Request, rawBody []byte) error {
	if req == nil || req.URL == nil {
		return &ValidationError{Field: "Request", Message: "must not be nil"}
	}
	appID := strings.TrimSpace(req.Header.Get(headerAppID))
	timestampText := strings.TrimSpace(req.Header.Get(headerTimestamp))
	nonce := strings.TrimSpace(req.Header.Get(headerNonce))
	signature := strings.TrimSpace(req.Header.Get(headerSignature))
	if appID == "" || timestampText == "" || nonce == "" || signature == "" {
		return ErrNotifyMissingHeader
	}
	if v.expectedAppID != "" && appID != v.expectedAppID {
		return ErrNotifyAppIDMismatch
	}
	timestamp, err := strconv.ParseInt(timestampText, 10, 64)
	if err != nil {
		return ErrNotifyInvalidTimestamp
	}
	now := v.now()
	signedAt := time.Unix(timestamp, 0)
	if signedAt.Before(now.Add(-v.maxClockSkew)) || signedAt.After(now.Add(v.maxClockSkew)) {
		return ErrNotifyInvalidTimestamp
	}
	path := v.signedPath
	if path == "" {
		path = req.URL.EscapedPath()
		if path == "" {
			path = "/"
		}
	}
	canonical := signing.Canonical(req.Method, path, timestamp, nonce, rawBody)
	if !signing.VerifyHex(v.secret, canonical, signature) {
		return ErrNotifyInvalidSignature
	}
	if v.replayStore != nil {
		ok, err := v.replayStore.Reserve(req.Context(), "notify:"+appID+":"+nonce, v.maxClockSkew*2)
		if err != nil {
			return fmt.Errorf("solopay: reserve notification nonce: %w", err)
		}
		if !ok {
			return ErrNotifyReplay
		}
	}
	return nil
}

// Parse verifies the callback before decoding it.
func (v *NotifyVerifier) Parse(req *http.Request, rawBody []byte) (*PaymentNotification, error) {
	if err := v.Verify(req, rawBody); err != nil {
		return nil, err
	}
	var payload wire.PaymentNotification
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, fmt.Errorf("solopay: decode notification: %w", err)
	}
	if payload.AppID != req.Header.Get(headerAppID) {
		return nil, ErrNotifyAppIDMismatch
	}
	paidAt, err := parseOptionalTime(payload.PaidAt, time.UTC)
	if err != nil {
		return nil, fmt.Errorf("solopay: decode notification paid_at: %w", err)
	}
	return &PaymentNotification{
		Event:               payload.Event,
		PlatformTradeNo:     payload.PlatformTradeNo,
		OutTradeNo:          payload.OutTradeNo,
		AppID:               payload.AppID,
		Status:              PaymentStatus(payload.Status),
		AmountCents:         payload.AmountCents,
		OriginalAmountCents: payload.OriginalAmountCents,
		DiscountCents:       payload.DiscountCents,
		MatchMode:           MatchMode(payload.MatchMode),
		Currency:            payload.Currency,
		Channel:             Channel(payload.ChannelType),
		Subject:             payload.ProductName,
		PaidAt:              paidAt,
		NotifyExtra:         cloneRawMessage(payload.NotifyExtra),
	}, nil
}

// MemoryReplayStore is suitable for a single application process. Distributed
// deployments should implement ReplayStore using their shared database/cache.
type MemoryReplayStore struct {
	mu      sync.Mutex
	entries map[string]time.Time
}

func NewMemoryReplayStore() *MemoryReplayStore {
	return &MemoryReplayStore{entries: make(map[string]time.Time)}
}

func (s *MemoryReplayStore) Reserve(_ context.Context, key string, ttl time.Duration) (bool, error) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries == nil {
		s.entries = make(map[string]time.Time)
	}
	for value, expiry := range s.entries {
		if !now.Before(expiry) {
			delete(s.entries, value)
		}
	}
	if _, exists := s.entries[key]; exists {
		return false, nil
	}
	s.entries[key] = now.Add(ttl)
	return true, nil
}
