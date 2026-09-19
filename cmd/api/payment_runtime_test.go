package main

import (
	"testing"

	"github.com/fourgeez/agentpay/internal/payments"
)

func TestValidatePaymentRuntimeConfigLocksDevelopmentToOfficialBaseSepoliaUSDC(t *testing.T) {
	t.Parallel()

	err := validatePaymentRuntimeConfig(paymentRuntimeConfig{
		Environment:    "dev",
		Mode:           "x402",
		FacilitatorURL: payments.TestnetFacilitatorURL,
		Network:        payments.BaseSepoliaNetwork,
		Asset:          payments.BaseSepoliaUSDCAsset,
	})
	if err != nil {
		t.Fatalf("validatePaymentRuntimeConfig() error = %v", err)
	}
}

func TestValidatePaymentRuntimeConfigAllowsMockOnlyForLocalDevelopment(t *testing.T) {
	t.Parallel()

	err := validatePaymentRuntimeConfig(paymentRuntimeConfig{
		Environment: "local",
		Mode:        "mock",
	})
	if err != nil {
		t.Fatalf("validatePaymentRuntimeConfig() error = %v", err)
	}

	if err := validatePaymentRuntimeConfig(paymentRuntimeConfig{
		Environment: "dev",
		Mode:        "mock",
	}); err == nil {
		t.Fatal("validatePaymentRuntimeConfig() error = nil, want non-local mock rejection")
	}
}

func TestValidatePaymentRuntimeConfigRejectsUnapprovedPaymentConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config paymentRuntimeConfig
	}{
		{
			name: "missing payment mode",
			config: paymentRuntimeConfig{
				Environment: "dev",
			},
		},
		{
			name: "production environment",
			config: paymentRuntimeConfig{
				Environment:    "prod",
				Mode:           "x402",
				FacilitatorURL: payments.TestnetFacilitatorURL,
				Network:        payments.BaseSepoliaNetwork,
				Asset:          payments.BaseSepoliaUSDCAsset,
			},
		},
		{
			name: "unapproved facilitator",
			config: paymentRuntimeConfig{
				Environment:    "dev",
				Mode:           "x402",
				FacilitatorURL: "https://facilitator.example",
				Network:        payments.BaseSepoliaNetwork,
				Asset:          payments.BaseSepoliaUSDCAsset,
			},
		},
		{
			name: "mainnet network",
			config: paymentRuntimeConfig{
				Environment:    "dev",
				Mode:           "x402",
				FacilitatorURL: payments.TestnetFacilitatorURL,
				Network:        "eip155:8453",
				Asset:          payments.BaseSepoliaUSDCAsset,
			},
		},
		{
			name: "unapproved asset",
			config: paymentRuntimeConfig{
				Environment:    "dev",
				Mode:           "x402",
				FacilitatorURL: payments.TestnetFacilitatorURL,
				Network:        payments.BaseSepoliaNetwork,
				Asset:          "0x1111111111111111111111111111111111111111",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := validatePaymentRuntimeConfig(test.config); err == nil {
				t.Fatal("validatePaymentRuntimeConfig() error = nil")
			}
		})
	}
}
