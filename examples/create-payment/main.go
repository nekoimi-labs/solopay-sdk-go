// Command create-payment creates a SoloPay order and prints its cashier URL.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	solopay "github.com/nekoimi-labs/solopay-sdk-go"
)

func main() {
	var (
		outTradeNo  = flag.String("out-trade-no", "", "unique merchant order number (required)")
		amountCents = flag.Int64("amount-cents", 0, "original amount in cents (required)")
		channel     = flag.String("channel", string(solopay.WeChatPay), "payment channel: wxpay or alipay")
		subject     = flag.String("subject", "", "order subject")
		notifyURL   = flag.String("notify-url", "", "payment notification URL (required)")
		returnURL   = flag.String("return-url", "", "customer return URL")
	)
	flag.Parse()

	if *outTradeNo == "" || *amountCents <= 0 || *notifyURL == "" {
		flag.Usage()
		os.Exit(2)
	}

	client, err := solopay.New(solopay.Config{
		BaseURL:   os.Getenv("SOLOPAY_BASE_URL"),
		AppID:     os.Getenv("SOLOPAY_APP_ID"),
		AppSecret: os.Getenv("SOLOPAY_APP_SECRET"),
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	payment, err := client.CreatePayment(ctx, solopay.CreatePaymentRequest{
		OutTradeNo:  *outTradeNo,
		Channel:     solopay.Channel(*channel),
		AmountCents: *amountCents,
		Currency:    "CNY",
		Subject:     *subject,
		NotifyURL:   *notifyURL,
		ReturnURL:   *returnURL,
	})
	if err != nil {
		var apiErr *solopay.APIError
		if errors.As(err, &apiErr) {
			log.Fatalf("SoloPay API error: http=%d code=%d request_id=%s message=%s",
				apiErr.HTTPStatus, apiErr.Code, apiErr.RequestID, apiErr.Message)
		}
		log.Fatal(err)
	}

	paymentURL, err := payment.PaymentURL()
	if err != nil {
		log.Fatalf("order %s was returned without a cashier URL: %v", payment.PlatformTradeNo, err)
	}
	fmt.Printf("platform_trade_no=%s\ncashier_url=%s\n", payment.PlatformTradeNo, paymentURL)
}
