package solopay

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultTimeout = 15 * time.Second

// Config configures a SoloPay client. A Client is immutable after creation and
// safe for concurrent use.
type Config struct {
	BaseURL   string
	AppID     string
	AppSecret string

	HTTPClient *http.Client
	UserAgent  string

	// ServerLocation is used for SoloPay's timezone-less order timestamps.
	// It defaults to UTC+8, matching the SoloPay default deployment timezone.
	ServerLocation *time.Location

	// AllowInsecureHTTP should only be enabled for local development.
	AllowInsecureHTTP bool
}

func normalizeConfig(cfg Config) (*url.URL, Config, error) {
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
	cfg.AppID = strings.TrimSpace(cfg.AppID)
	if cfg.BaseURL == "" {
		return nil, cfg, fmt.Errorf("solopay: BaseURL is required")
	}
	if cfg.AppID == "" {
		return nil, cfg, fmt.Errorf("solopay: AppID is required")
	}
	if cfg.AppSecret == "" {
		return nil, cfg, fmt.Errorf("solopay: AppSecret is required")
	}

	base, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, cfg, fmt.Errorf("solopay: parse BaseURL: %w", err)
	}
	if base.Scheme != "https" && !(cfg.AllowInsecureHTTP && base.Scheme == "http") {
		return nil, cfg, fmt.Errorf("solopay: BaseURL must use HTTPS")
	}
	if base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, cfg, fmt.Errorf("solopay: BaseURL must contain only scheme and host")
	}
	if base.Path != "" && base.Path != "/" {
		return nil, cfg, fmt.Errorf("solopay: BaseURL must not contain a path")
	}
	base.Path = ""
	base.RawPath = ""

	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: defaultTimeout}
	}
	if strings.TrimSpace(cfg.UserAgent) == "" {
		cfg.UserAgent = defaultUserAgent
	} else {
		cfg.UserAgent = defaultUserAgent + " " + strings.TrimSpace(cfg.UserAgent)
	}
	if cfg.ServerLocation == nil {
		cfg.ServerLocation = time.FixedZone("UTC+8", 8*60*60)
	}
	return base, cfg, nil
}
