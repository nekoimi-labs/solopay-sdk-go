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

// CreatePaymentRequest contains the business fields needed to create a
// payment. AmountCents is the merchant's business order amount in the
// currency's smallest unit. It is not the actual payment amount when SoloPay
// applies a random discount.
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

// Payment is returned by create and query operations. AmountCents is always
// the merchant's business order amount. Status includes the current state on
// create replays, including terminal states. CashierURL can be restored by an
// idempotent create replay or an order query while pending and unexpired.
type Payment struct {
	IdempotentReplay bool
	OrderID          int64
	PlatformTradeNo  string
	OutTradeNo       string
	Channel          Channel
	Status           PaymentStatus
	Currency         string

	AmountCents int64

	Subject      string
	ConfirmCode  string
	CashierURL   string
	CashierToken string
	QRURL        string

	ExpireAt  time.Time
	PaidAt    *time.Time
	CreatedAt time.Time
}

var ErrCashierURLUnavailable = errors.New("solopay: cashier URL is unavailable for this payment")

// PaymentCancellation is returned after a merchant requests order cancellation.
type PaymentCancellation struct {
	PlatformTradeNo string
	OutTradeNo      string
	Status          PaymentStatus
}

// PaymentURL returns the URL to which the customer should be redirected.
func (p *Payment) PaymentURL() (string, error) {
	if p == nil || p.CashierURL == "" {
		return "", ErrCashierURLUnavailable
	}
	return p.CashierURL, nil
}
