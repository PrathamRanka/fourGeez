package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fourgeez/agentpay/internal/payments"
)

const (
	paymentModeMock = "mock"
	paymentModeX402 = "x402"
)

type paymentRuntimeConfig struct {
	Environment    string
	Mode           string
	FacilitatorURL string
	Network        string
	Asset          string
}

func validatePaymentRuntimeConfig(config paymentRuntimeConfig) error {
	environment := strings.TrimSpace(config.Environment)
	mode := strings.TrimSpace(config.Mode)

	switch mode {
	case paymentModeMock:
		if environment != "local" {
			return errors.New("mock payments are allowed only in the local environment")
		}
		return nil
	case paymentModeX402:
		if environment == "prod" {
			return errors.New("production payments are disabled; x402 is limited to testnet environments")
		}
		if strings.TrimSpace(config.FacilitatorURL) != payments.TestnetFacilitatorURL {
			return fmt.Errorf("facilitator must be the approved x402 testnet endpoint %q", payments.TestnetFacilitatorURL)
		}
		if strings.TrimSpace(config.Network) != payments.BaseSepoliaNetwork {
			return fmt.Errorf("network must be Base Sepolia %q", payments.BaseSepoliaNetwork)
		}
		if !strings.EqualFold(strings.TrimSpace(config.Asset), payments.BaseSepoliaUSDCAsset) {
			return fmt.Errorf("asset must be Base Sepolia USDC %q", payments.BaseSepoliaUSDCAsset)
		}
		return nil
	default:
		return errors.New("AGENTPAY_PAYMENT_MODE must be either mock or x402")
	}
}
