package solopay

import (
	"encoding/json"
	"errors"
	"fmt"
)

// APIError represents a response rejected by SoloPay at either the HTTP or
// business-protocol layer.
type APIError struct {
	HTTPStatus int
	Code       int
	Message    string
	Detail     json.RawMessage
	RequestID  string
}

func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Code != 0 {
		return fmt.Sprintf("solopay: API error: http=%d code=%d message=%s", e.HTTPStatus, e.Code, e.Message)
	}
	return fmt.Sprintf("solopay: HTTP error: status=%d message=%s", e.HTTPStatus, e.Message)
}

// IsAPIErrorCode reports whether err contains an APIError with code.
func IsAPIErrorCode(err error, code int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == code
}

// DecodeError means SoloPay returned a response that did not match its public
// JSON envelope contract.
type DecodeError struct {
	HTTPStatus int
	Err        error
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("solopay: decode response with HTTP status %d: %v", e.HTTPStatus, e.Err)
}

func (e *DecodeError) Unwrap() error { return e.Err }

// ValidationError reports an invalid SDK input before a request is sent.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("solopay: invalid %s: %s", e.Field, e.Message)
}
