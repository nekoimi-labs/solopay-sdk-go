package wire

import "encoding/json"

type Envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Error   json.RawMessage `json:"error"`
}

type CreatePaymentRequest struct {
	OutTradeNo  string `json:"out_trade_no"`
	ChannelType string `json:"channel_type"`
	AmountCents int64  `json:"amount_cents,string"`
	Currency    string `json:"currency,omitempty"`
	ProductName string `json:"product_name,omitempty"`
	NotifyURL   string `json:"notify_url"`
	ReturnURL   string `json:"return_url,omitempty"`
}

type Payment struct {
	IdempotentReplay    bool    `json:"idempotent_replay,omitempty"`
	OrderID             int64   `json:"order_id,string"`
	PlatformTradeNo     string  `json:"platform_trade_no"`
	OutTradeNo          string  `json:"out_trade_no"`
	ChannelType         string  `json:"channel_type"`
	AmountCents         int64   `json:"amount_cents,string"`
	OriginalAmountCents int64   `json:"original_amount_cents,string"`
	DiscountCents       int64   `json:"discount_cents,string"`
	MatchMode           string  `json:"match_mode"`
	Currency            string  `json:"currency"`
	Status              string  `json:"status"`
	ConfirmCode         string  `json:"confirm_code,omitempty"`
	ProductName         string  `json:"product_name,omitempty"`
	PaidAt              *string `json:"paid_at,omitempty"`
	ExpireAt            string  `json:"expire_at"`
	CreatedAt           string  `json:"created_at,omitempty"`
	CashierURL          string  `json:"cashier_url,omitempty"`
	CashierToken        string  `json:"cashier_token,omitempty"`
	QRURL               string  `json:"qr_url,omitempty"`
}

type PaymentNotification struct {
	Event               string          `json:"event"`
	PlatformTradeNo     string          `json:"platform_trade_no"`
	OutTradeNo          string          `json:"out_trade_no"`
	AppID               string          `json:"app_id"`
	Status              string          `json:"status"`
	AmountCents         int64           `json:"amount_cents,string"`
	OriginalAmountCents int64           `json:"original_amount_cents,string"`
	DiscountCents       int64           `json:"discount_cents,string"`
	MatchMode           string          `json:"match_mode"`
	Currency            string          `json:"currency"`
	ChannelType         string          `json:"channel_type"`
	ProductName         string          `json:"product_name,omitempty"`
	PaidAt              *string         `json:"paid_at,omitempty"`
	NotifyExtra         json.RawMessage `json:"notify_extra,omitempty"`
}
