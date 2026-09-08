# SoloPay Go SDK

SoloPay 的 Go 支付接入 SDK，面向业务服务端，提供快捷下单、获取收银台地址、订单查询和支付回调验签。

SDK 使用 Go 标准库实现，要求 Go 1.22 或更高版本。

## 安装

```bash
go get github.com/nekoimi/solopay-sdk-go@latest
```

尚未发布版本标签时，可临时使用 `@master`，或在业务项目的 `go.mod` 中使用本地 `replace`。

## 快速下单

应用的 `app_id` 和密钥应仅保存在业务服务端，不要放进浏览器、App 或小程序。

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	solopay "github.com/nekoimi/solopay-sdk-go"
)

func main() {
	client, err := solopay.New(solopay.Config{
		BaseURL:   os.Getenv("SOLOPAY_BASE_URL"),
		AppID:     os.Getenv("SOLOPAY_APP_ID"),
		AppSecret: os.Getenv("SOLOPAY_APP_SECRET"),
	})
	if err != nil {
		log.Fatal(err)
	}

	payment, err := client.CreatePayment(context.Background(), solopay.CreatePaymentRequest{
		OutTradeNo:  "ORDER-20260908-001",
		Channel:     solopay.WeChatPay,
		AmountCents: 100, // 原价 1.00 CNY，单位为分
		Currency:    "CNY",
		Subject:     "测试商品",
		NotifyURL:   "https://merchant.example.com/webhooks/solopay",
		ReturnURL:   "https://merchant.example.com/orders/ORDER-20260908-001",
	})
	if err != nil {
		log.Fatal(err)
	}

	paymentURL, err := payment.PaymentURL()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(payment.PlatformTradeNo, paymentURL)
}
```

在 HTTP Handler 中可将客户跳转到收银台：

```go
paymentURL, err := payment.PaymentURL()
if err != nil {
	http.Error(w, "cashier unavailable", http.StatusBadGateway)
	return
}
http.Redirect(w, r, paymentURL, http.StatusSeeOther)
```

`CreatePayment` 自动生成 HMAC 请求头。未设置 `IdempotencyKey` 时，SDK 默认使用 `OutTradeNo`，因此超时后可使用完全相同的业务订单号和请求参数重新调用。

> SoloPay 的幂等重放响应不会再次返回收银台 token 或 `cashier_url`。业务系统必须持久化首次下单成功返回的 `CashierURL`；重放时 `PaymentURL()` 会返回 `ErrCashierURLUnavailable`。

## 查询订单

```go
// 使用 SoloPay 平台订单号查询。
payment, err := client.GetPayment(ctx, "202609080001")

// 或使用业务订单号查询。
payment, err = client.GetPaymentByOutTradeNo(ctx, "ORDER-20260908-001")

if err == nil && (payment.Status == solopay.PaymentPaid ||
	payment.Status == solopay.PaymentManualPaid) {
	// 处理支付成功；业务侧仍需保证自身处理幂等。
}
```

请求中的 `AmountCents` 是商品原价。响应中的 `Payment.AmountCents` 是客户实际应付金额；`OriginalAmountCents` 是原价，`DiscountCents` 是优惠金额。

## 错误处理

网络错误保留在错误链中，可使用 `errors.Is` / `errors.As` 判断。SoloPay 返回的 HTTP 或业务错误为 `*solopay.APIError`：

```go
payment, err := client.GetPayment(ctx, platformTradeNo)
if err != nil {
	var apiErr *solopay.APIError
	if errors.As(err, &apiErr) {
		log.Printf("solopay rejected request: http=%d code=%d request_id=%s message=%s",
			apiErr.HTTPStatus, apiErr.Code, apiErr.RequestID, apiErr.Message)
		return
	}
	log.Printf("solopay request failed: %v", err)
}
```

无效输入会返回 `*solopay.ValidationError`，不匹配协议的响应会返回 `*solopay.DecodeError`。

## 支付回调验签

回调必须使用读取到的原始请求体验签，不能先反序列化再重新编码。配置了多实例部署时，应使用 Redis 或数据库实现 `ReplayStore`；`MemoryReplayStore` 只适合单进程。

```go
verifier, err := solopay.NewNotifyVerifier(solopay.NotifyVerifierConfig{
	Secret:        os.Getenv("SOLOPAY_NOTIFY_SIGNING_SECRET"),
	ExpectedAppID: os.Getenv("SOLOPAY_APP_ID"),
	ReplayStore:   solopay.NewMemoryReplayStore(),
})
if err != nil {
	log.Fatal(err)
}

http.HandleFunc("/webhooks/solopay", func(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	notification, err := verifier.Parse(r, body)
	if err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	// 按 notification.OutTradeNo 幂等更新业务订单，并核对状态和金额。
	_ = notification
	w.WriteHeader(http.StatusNoContent)
})
```

SoloPay 使用回调 URL 的转义路径参与签名。如果反向代理改写了应用收到的路径，请在 `NotifyVerifierConfig.SignedPath` 中设置 SoloPay 实际请求的原始路径。

## 配置说明

- `BaseURL`：SoloPay 服务根地址，不含路径、查询参数或片段；默认只允许 HTTPS。
- `AppID` / `AppSecret`：管理后台应用配置中的开放 API 凭据。
- `HTTPClient`：可选的自定义 HTTP 客户端；未设置时超时为 15 秒。
- `ServerLocation`：用于解释接口中不带时区的订单时间，默认 UTC+8。
- `AllowInsecureHTTP`：仅用于本地开发和测试。

`NotifyURL` 和 `ReturnURL` 的最终安全策略由 SoloPay 服务端配置决定。生产环境应使用公网可访问的 HTTPS 地址。

完整命令行示例见 [`examples/create-payment`](examples/create-payment)。

## 许可证

本项目使用 [GNU Affero General Public License v3.0](LICENSE)。
