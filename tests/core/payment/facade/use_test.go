package payment_test

import (
	"context"
	"testing"

	"github.com/huwenlong92/sdkit-go/core/payment"
	paymentfacade "github.com/huwenlong92/sdkit-go/core/payment/facade"
	"github.com/huwenlong92/sdkit-go/core/runtime"
	"github.com/huwenlong92/sdkit-go/pkg/payment/mock"
)

func TestUseWithSetupRegistersDefaultService(t *testing.T) {
	t.Cleanup(payment.Close)

	app := runtime.New()

	t.Cleanup(func() { _ = app.Stop(context.Background()) })
	capability := paymentfacade.Use(
		paymentfacade.WithSetup(func(app *runtime.App, registry *payment.Registry) error {
			return registry.Register(mock.New(
				payment.ProviderStripe,
				mock.WithChannels(payment.ChannelStripeCheckout),
				mock.WithCurrencies("USD"),
				mock.WithActions(payment.ActionRedirectURL),
				mock.WithAction(payment.PaymentAction{
					Type: payment.ActionRedirectURL,
					URL:  "https://pay.example.test/checkout",
				}),
			))
		}),
		paymentfacade.WithChannelBindings(payment.ChannelBinding{
			Key:         "school_a_p",
			Provider:    payment.ProviderStripe,
			Channel:     payment.ChannelStripeCheckout,
			MerchantKey: "school_a",
		}),
	)

	if err := capability.Register(app); err != nil {
		t.Fatalf("register capability: %v", err)
	}
	if got := paymentfacade.From(app); got == nil {
		t.Fatalf("facade from app = nil")
	}

	resp, err := payment.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		MerchantKey: "school_a_p",
		OutTradeNo:  "pay_1001",
		Pricing: payment.PaymentPricing{
			PayAmount:      payment.Money{Amount: 1200, Currency: "USD"},
			SettleCurrency: "USD",
		},
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if resp.MerchantKey != "school_a" || resp.Action.Type != payment.ActionRedirectURL {
		t.Fatalf("response = %+v", resp)
	}
	if resp.Provider != payment.ProviderStripe || resp.Channel != payment.ChannelStripeCheckout {
		t.Fatalf("selected channel = %s/%s", resp.Provider, resp.Channel)
	}
}

func TestUseLoadsChannelBindingsFromConfigLoader(t *testing.T) {
	t.Cleanup(payment.Close)

	app := runtime.New()

	t.Cleanup(func() { _ = app.Stop(context.Background()) })
	capability := paymentfacade.Use(
		paymentfacade.WithConfigLoader(func(app *runtime.App) (paymentfacade.Config, error) {
			return paymentfacade.Config{
				Channels: []payment.ChannelBinding{
					{
						Key:         "school_a_p",
						Provider:    payment.ProviderStripe,
						Channel:     payment.ChannelStripeCheckout,
						MerchantKey: "school_a",
					},
				},
			}, nil
		}),
		paymentfacade.WithSetup(func(app *runtime.App, registry *payment.Registry) error {
			return registry.Register(mock.New(
				payment.ProviderStripe,
				mock.WithChannels(payment.ChannelStripeCheckout),
				mock.WithCurrencies("USD"),
				mock.WithActions(payment.ActionRedirectURL),
				mock.WithAction(payment.PaymentAction{
					Type: payment.ActionRedirectURL,
					URL:  "https://pay.example.test/checkout",
				}),
			))
		}),
	)

	if err := capability.Register(app); err != nil {
		t.Fatalf("register capability: %v", err)
	}

	resp, err := payment.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		MerchantKey: "school_a_p",
		OutTradeNo:  "pay_1001",
		Pricing: payment.PaymentPricing{
			PayAmount:      payment.Money{Amount: 1200, Currency: "USD"},
			SettleCurrency: "USD",
		},
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if resp.Provider != payment.ProviderStripe ||
		resp.Channel != payment.ChannelStripeCheckout ||
		resp.MerchantKey != "school_a" {
		t.Fatalf("response = %+v", resp)
	}
}
