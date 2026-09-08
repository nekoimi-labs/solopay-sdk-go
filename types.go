package solopay

import (
	"errors"
	"time"
)

// Channel identifies the payment channel accepted by SoloPay.
type Channel string

const (
	WeChatPay Channel = "wxpay"
	Alipay    Channel = "alipay"
)

// PaymentStatus is an order state. Unknown future values remain usable as a
// string so older SDK versions can continue decoding newer server responses.
type PaymentStatus string

const (
	PaymentPending    PaymentStatus = "pending"
	PaymentPaid       PaymentStatus = "paid"
	PaymentManualPaid PaymentStatus = "manual_paid"
	PaymentExpired    PaymentStatus = "expired"
	PaymentClosed     PaymentStatus = "closed"
)

type MatchMode string

const (
	MatchConfirmCode    MatchMode = "confirm_code"
	MatchRandomDiscount MatchMode = "random_discount"
)

// CreatePaymentRequest contains the business fields needed to create a
// payment. AmountCents uses the currency's smallest unit.
type CreatePaymentRequest struct {
	OutTradeNo     string
	Channel        Channel
	AmountCents    int64
	Currency       string
	Subject        string
	NotifyURL      string
	ReturnURL      string
	IdempotencyKey string
}

// Payment is returned by both create and query operations. CashierURL,
// CashierToken, QRURL and ConfirmCode are normally populated only by the first
// successful create response.
type Payment struct {
	IdempotentReplay bool
	OrderID          int64
	PlatformTradeNo  string
	OutTradeNo       string
	Channel          Channel
	Status           PaymentStatus
	Currency         string

	AmountCents         int64
	OriginalAmountCents int64
	DiscountCents       int64

	Subject      string
	MatchMode    MatchMode
	ConfirmCode  string
	CashierURL   string
	CashierToken string
	QRURL        string

	ExpireAt  time.Time
	PaidAt    *time.Time
	CreatedAt time.Time
}

var ErrCashierURLUnavailable = errors.New("solopay: cashier URL is unavailable on an idempotent replay")

// PaymentURL returns the URL to which the customer should be redirected.
func (p *Payment) PaymentURL() (string, error) {
	if p == nil || p.CashierURL == "" {
		return "", ErrCashierURLUnavailable
	}
	return p.CashierURL, nil
}
