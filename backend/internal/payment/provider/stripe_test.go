//go:build unit

package provider

import (
	"bytes"
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
	stripe "github.com/stripe/stripe-go/v85"
)

type stripeRefundBackend struct {
	params []*stripe.RefundCreateParams
}

func (b *stripeRefundBackend) Call(_ string, _ string, _ string, params stripe.ParamsContainer, v stripe.LastResponseSetter) error {
	b.params = append(b.params, params.(*stripe.RefundCreateParams))
	refund := v.(*stripe.Refund)
	refund.ID = "re_123"
	refund.Status = stripe.RefundStatusSucceeded
	return nil
}

func (*stripeRefundBackend) CallStreaming(string, string, string, stripe.ParamsContainer, stripe.StreamingLastResponseSetter) error {
	return nil
}

func (*stripeRefundBackend) CallRaw(string, string, string, []byte, *stripe.Params, stripe.LastResponseSetter) error {
	return nil
}

func (*stripeRefundBackend) CallMultipart(string, string, string, string, *bytes.Buffer, *stripe.Params, stripe.LastResponseSetter) error {
	return nil
}

func (*stripeRefundBackend) SetMaxNetworkRetries(int64) {}

func TestStripeRefundUsesStableAmountSpecificIdempotencyKey(t *testing.T) {
	backend := &stripeRefundBackend{}
	client := stripe.NewClient("sk_test", stripe.WithBackends(&stripe.Backends{API: backend}))
	provider := &Stripe{
		config:      map[string]string{"currency": "CNY"},
		initialized: true,
		sc:          client,
	}

	refund := func(amount string) {
		_, err := provider.Refund(context.Background(), payment.RefundRequest{
			TradeNo: "pi_123",
			OrderID: "sub2_order_456",
			Amount:  amount,
		})
		require.NoError(t, err)
	}

	refund("12.34")
	refund("12.34")
	refund("12.35")

	require.Len(t, backend.params, 3)
	require.Equal(t, int64(1234), *backend.params[0].Amount)
	require.Equal(t, "re-sub2_order_456-1234", *backend.params[0].IdempotencyKey)
	require.Equal(t, backend.params[0].IdempotencyKey, backend.params[1].IdempotencyKey)
	require.Equal(t, int64(1235), *backend.params[2].Amount)
	require.Equal(t, "re-sub2_order_456-1235", *backend.params[2].IdempotencyKey)
	require.NotEqual(t, *backend.params[0].IdempotencyKey, *backend.params[2].IdempotencyKey)

}

func TestCreatePaymentCardUsesAutomaticPaymentMethods(t *testing.T) {
	req := payment.CreatePaymentRequest{
		OrderID:            "test-order-card",
		Amount:             "100.00",
		PaymentType:        "card",
		Subject:            "Test",
		InstanceSubMethods: "card",
	}
	params, err := buildPaymentIntentParams(req, payment.DefaultPaymentCurrency)
	if err != nil {
		t.Fatalf("buildPaymentIntentParams: %v", err)
	}
	if params.AutomaticPaymentMethods == nil {
		t.Fatal("expected AutomaticPaymentMethods to be set for PaymentType=card")
	}
	if params.AutomaticPaymentMethods.Enabled == nil || !*params.AutomaticPaymentMethods.Enabled {
		t.Errorf("AutomaticPaymentMethods.Enabled should be true")
	}
	if len(params.PaymentMethodTypes) != 0 {
		t.Errorf("PaymentMethodTypes should be empty when automatic methods enabled, got %d entries", len(params.PaymentMethodTypes))
	}
}

func TestCreatePaymentAlipayUsesExplicitMethodTypes(t *testing.T) {
	req := payment.CreatePaymentRequest{
		OrderID:            "test-order-alipay",
		Amount:             "100.00",
		PaymentType:        "alipay",
		Subject:            "Test",
		InstanceSubMethods: "alipay",
	}
	params, err := buildPaymentIntentParams(req, payment.DefaultPaymentCurrency)
	if err != nil {
		t.Fatalf("buildPaymentIntentParams: %v", err)
	}
	if params.AutomaticPaymentMethods != nil {
		t.Errorf("AutomaticPaymentMethods should be nil for alipay")
	}
	if len(params.PaymentMethodTypes) != 1 || *params.PaymentMethodTypes[0] != "alipay" {
		t.Errorf("PaymentMethodTypes should be [alipay], got %+v", params.PaymentMethodTypes)
	}
}

func TestCreatePaymentWxpayUsesExplicitMethodTypesAndOptions(t *testing.T) {
	req := payment.CreatePaymentRequest{
		OrderID:            "test-order-wxpay",
		Amount:             "100.00",
		PaymentType:        "wxpay",
		Subject:            "Test",
		InstanceSubMethods: "wxpay",
	}
	params, err := buildPaymentIntentParams(req, payment.DefaultPaymentCurrency)
	if err != nil {
		t.Fatalf("buildPaymentIntentParams: %v", err)
	}
	if params.AutomaticPaymentMethods != nil {
		t.Errorf("AutomaticPaymentMethods should be nil for wxpay")
	}
	if len(params.PaymentMethodTypes) != 1 || *params.PaymentMethodTypes[0] != "wechat_pay" {
		t.Errorf("PaymentMethodTypes should be [wechat_pay], got %+v", params.PaymentMethodTypes)
	}
	if params.PaymentMethodOptions == nil || params.PaymentMethodOptions.WeChatPay == nil {
		t.Fatal("expected PaymentMethodOptions.WeChatPay to be set")
	}
	if params.PaymentMethodOptions.WeChatPay.Client == nil || *params.PaymentMethodOptions.WeChatPay.Client != "web" {
		t.Errorf("WeChatPay.Client should be 'web'")
	}
}
