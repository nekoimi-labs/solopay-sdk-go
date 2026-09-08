package solopay

import (
	"fmt"
	"strings"
	"time"
)

const serverTimeLayout = "2006-01-02 15:04:05"

func parseServerTime(value string, location *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	if parsed, err := time.ParseInLocation(serverTimeLayout, value, location); err == nil {
		return parsed, nil
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("solopay: unsupported time %q", value)
}

func parseOptionalTime(value *string, location *time.Location) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	parsed, err := parseServerTime(*value, location)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
