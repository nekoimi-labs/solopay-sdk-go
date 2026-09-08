package solopay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/nekoimi/solopay-sdk-go/internal/wire"
)

// CreatePayment creates a SoloPay order and returns the customer-facing
// cashier URL. It deliberately does not automatically retry ambiguous network
// failures; retry with the same request instead.
func (c *Client) CreatePayment(ctx context.Context, input CreatePaymentRequest) (*Payment, error) {
	input, err := validateCreatePayment(input)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(wire.CreatePaymentRequest{
		OutTradeNo:  input.OutTradeNo,
		ChannelType: string(input.Channel),
		AmountCents: input.AmountCents,
		Currency:    input.Currency,
		ProductName: input.Subject,
		NotifyURL:   input.NotifyURL,
		ReturnURL:   input.ReturnURL,
	})
	if err != nil {
		return nil, fmt.Errorf("solopay: encode payment request: %w", err)
	}
	var result wire.Payment
	if err := c.do(ctx, http.MethodPost, "/v1/open/orders", nil, body, input.IdempotencyKey, &result); err != nil {
		return nil, err
	}
	payment, err := c.paymentFromWire(result)
	if err != nil {
		return nil, err
	}
	return payment, nil
}

// CreateOrder is a compatibility alias for CreatePayment.
func (c *Client) CreateOrder(ctx context.Context, input CreatePaymentRequest) (*Payment, error) {
	return c.CreatePayment(ctx, input)
}

// GetPayment queries a payment by its SoloPay platform trade number.
func (c *Client) GetPayment(ctx context.Context, platformTradeNo string) (*Payment, error) {
	platformTradeNo = strings.TrimSpace(platformTradeNo)
	if platformTradeNo == "" || len(platformTradeNo) > 64 {
		return nil, &ValidationError{Field: "PlatformTradeNo", Message: "must contain 1 to 64 characters"}
	}
	path := "/v1/open/orders/" + url.PathEscape(platformTradeNo)
	var result wire.Payment
	if err := c.do(ctx, http.MethodGet, path, nil, nil, "", &result); err != nil {
		return nil, err
	}
	return c.paymentFromWire(result)
}

// GetOrder is a compatibility alias for GetPayment.
func (c *Client) GetOrder(ctx context.Context, platformTradeNo string) (*Payment, error) {
	return c.GetPayment(ctx, platformTradeNo)
}

// GetPaymentByOutTradeNo queries a payment by the merchant's trade number.
func (c *Client) GetPaymentByOutTradeNo(ctx context.Context, outTradeNo string) (*Payment, error) {
	outTradeNo = strings.TrimSpace(outTradeNo)
	if outTradeNo == "" || len(outTradeNo) > 128 {
		return nil, &ValidationError{Field: "OutTradeNo", Message: "must contain 1 to 128 characters"}
	}
	query := url.Values{"out_trade_no": []string{outTradeNo}}
	var result wire.Payment
	if err := c.do(ctx, http.MethodGet, "/v1/open/orders/by-out-trade-no", query, nil, "", &result); err != nil {
		return nil, err
	}
	return c.paymentFromWire(result)
}

// GetOrderByOutTradeNo is a compatibility alias for GetPaymentByOutTradeNo.
func (c *Client) GetOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*Payment, error) {
	return c.GetPaymentByOutTradeNo(ctx, outTradeNo)
}

func validateCreatePayment(input CreatePaymentRequest) (CreatePaymentRequest, error) {
	input.OutTradeNo = strings.TrimSpace(input.OutTradeNo)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.Subject = strings.TrimSpace(input.Subject)
	input.NotifyURL = strings.TrimSpace(input.NotifyURL)
	input.ReturnURL = strings.TrimSpace(input.ReturnURL)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.OutTradeNo == "" || len(input.OutTradeNo) > 128 {
		return input, &ValidationError{Field: "OutTradeNo", Message: "must contain 1 to 128 characters"}
	}
	if input.Channel != WeChatPay && input.Channel != Alipay {
		return input, &ValidationError{Field: "Channel", Message: "must be solopay.WeChatPay or solopay.Alipay"}
	}
	if input.AmountCents <= 0 {
		return input, &ValidationError{Field: "AmountCents", Message: "must be greater than zero"}
	}
	if input.Currency == "" {
		input.Currency = "CNY"
	}
	if len(input.Currency) != 3 {
		return input, &ValidationError{Field: "Currency", Message: "must contain exactly 3 characters"}
	}
	if len(input.Subject) > 256 {
		return input, &ValidationError{Field: "Subject", Message: "must not exceed 256 characters"}
	}
	if err := validateAbsoluteHTTPURL("NotifyURL", input.NotifyURL, true); err != nil {
		return input, err
	}
	if input.ReturnURL != "" {
		if err := validateAbsoluteHTTPURL("ReturnURL", input.ReturnURL, false); err != nil {
			return input, err
		}
	}
	if input.IdempotencyKey == "" {
		input.IdempotencyKey = input.OutTradeNo
	}
	if len(input.IdempotencyKey) > 128 {
		return input, &ValidationError{Field: "IdempotencyKey", Message: "must not exceed 128 characters"}
	}
	return input, nil
}

func validateAbsoluteHTTPURL(field, value string, required bool) error {
	if value == "" {
		if required {
			return &ValidationError{Field: field, Message: "is required"}
		}
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return &ValidationError{Field: field, Message: "must be an absolute HTTP(S) URL"}
	}
	return nil
}

func (c *Client) paymentFromWire(input wire.Payment) (*Payment, error) {
	expireAt, err := parseServerTime(input.ExpireAt, c.location)
	if err != nil {
		return nil, &DecodeError{HTTPStatus: http.StatusOK, Err: fmt.Errorf("expire_at: %w", err)}
	}
	paidAt, err := parseOptionalTime(input.PaidAt, c.location)
	if err != nil {
		return nil, &DecodeError{HTTPStatus: http.StatusOK, Err: fmt.Errorf("paid_at: %w", err)}
	}
	createdAt, err := parseServerTime(input.CreatedAt, c.location)
	if err != nil {
		return nil, &DecodeError{HTTPStatus: http.StatusOK, Err: fmt.Errorf("created_at: %w", err)}
	}
	return &Payment{
		IdempotentReplay:    input.IdempotentReplay,
		OrderID:             input.OrderID,
		PlatformTradeNo:     input.PlatformTradeNo,
		OutTradeNo:          input.OutTradeNo,
		Channel:             Channel(input.ChannelType),
		Status:              PaymentStatus(input.Status),
		Currency:            input.Currency,
		AmountCents:         input.AmountCents,
		OriginalAmountCents: input.OriginalAmountCents,
		DiscountCents:       input.DiscountCents,
		Subject:             input.ProductName,
		MatchMode:           MatchMode(input.MatchMode),
		ConfirmCode:         input.ConfirmCode,
		CashierURL:          input.CashierURL,
		CashierToken:        input.CashierToken,
		QRURL:               input.QRURL,
		ExpireAt:            expireAt,
		PaidAt:              paidAt,
		CreatedAt:           createdAt,
	}, nil
}
