package payments

import "testing"

func TestX402AdapterAdvertisesOnlyEnabledTestnetCapability(t *testing.T) {
	t.Parallel()

	catalog := NewX402Adapter().PaymentCapabilities()
	if catalog.SchemaVersion != PaymentCapabilitySchemaVersion ||
		catalog.Environment != PaymentEnvironmentTestnet ||
		catalog.SelectionRule != PaymentSelectionFirstCompatible {
		t.Fatalf("catalog = %#v", catalog)
	}
	if len(catalog.Capabilities) != 1 {
		t.Fatalf("capabilities = %#v", catalog.Capabilities)
	}
	capability := catalog.Capabilities[0]
	if capability.CapabilityID != X402BaseSepoliaUSDCCapabilityID ||
		capability.Rail != PaymentRailX402 ||
		capability.Scheme != ExactScheme ||
		capability.Network != BaseSepoliaNetwork ||
		capability.Asset.Identifier != BaseSepoliaUSDCAsset ||
		capability.Asset.Symbol != "USDC" ||
		capability.Asset.Decimals != 6 ||
		capability.Settlement != PaymentSettlementDirectToSeller ||
		capability.Custody ||
		capability.Wallet == nil ||
		capability.Wallet.ChainID != 84532 ||
		capability.Wallet.ChainIDHex != "0x14a34" {
		t.Fatalf("capability = %#v", capability)
	}
	if len(capability.Channels) != 2 ||
		capability.Channels[0] != PaymentChannelAgent ||
		capability.Channels[1] != PaymentChannelBrowser {
		t.Fatalf("channels = %#v", capability.Channels)
	}
}

func TestMockAdapterAdvertisesOnlyLocalMockCapability(t *testing.T) {
	t.Parallel()

	catalog := NewMockAdapter().PaymentCapabilities()
	if catalog.Environment != PaymentEnvironmentLocal || len(catalog.Capabilities) != 1 {
		t.Fatalf("catalog = %#v", catalog)
	}
	capability := catalog.Capabilities[0]
	if capability.Rail != PaymentRailMock || capability.Wallet != nil {
		t.Fatalf("capability = %#v", capability)
	}
}

func TestDetectExternalBuyerCompatibilitySelectsSupportedX402Capability(t *testing.T) {
	t.Parallel()

	result := DetectExternalBuyerCompatibility(
		NewX402Adapter().PaymentCapabilities(),
		ExternalBuyerCompatibilityRequest{
			SchemaVersion: ExternalBuyerCapabilitiesSchemaVersion,
			Capabilities: []ExternalBuyerPaymentCapability{{
				Protocol:    PaymentRailX402,
				X402Version: 2,
				Scheme:      ExactScheme,
				Network:     BaseSepoliaNetwork,
				Asset:       BaseSepoliaUSDCAsset,
			}},
		},
	)

	if !result.Compatible || result.ReasonCode != ExternalBuyerCompatibleReason ||
		result.SelectedCapability == nil ||
		result.SelectedCapability.CapabilityID != X402BaseSepoliaUSDCCapabilityID {
		t.Fatalf("compatibility result = %#v", result)
	}
	if result.Authentication.HeaderName != "X-AgentPay-Agent-Key" ||
		result.Authentication.SellerProjectKeyAccepted {
		t.Fatalf("authentication = %#v", result.Authentication)
	}
	if result.Headers.PaymentRequired != "PAYMENT-REQUIRED" ||
		result.Headers.PaymentSignature != "PAYMENT-SIGNATURE" ||
		result.Headers.PaymentResponse != "PAYMENT-RESPONSE" ||
		result.Headers.IntentID != "X-AgentPay-Intent-Id" ||
		result.Headers.TransactionID != "X-AgentPay-Transaction-Id" {
		t.Fatalf("headers = %#v", result.Headers)
	}
}

func TestDetectExternalBuyerCompatibilityRejectsUnsupportedOrLocalOnlyCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		catalog PaymentCapabilityCatalog
		request ExternalBuyerCompatibilityRequest
	}{
		{
			name:    "unsupported x402 version",
			catalog: NewX402Adapter().PaymentCapabilities(),
			request: ExternalBuyerCompatibilityRequest{
				SchemaVersion: ExternalBuyerCapabilitiesSchemaVersion,
				Capabilities: []ExternalBuyerPaymentCapability{{
					Protocol: PaymentRailX402, X402Version: 1, Scheme: ExactScheme,
					Network: BaseSepoliaNetwork, Asset: BaseSepoliaUSDCAsset,
				}},
			},
		},
		{
			name:    "unsupported network",
			catalog: NewX402Adapter().PaymentCapabilities(),
			request: ExternalBuyerCompatibilityRequest{
				SchemaVersion: ExternalBuyerCapabilitiesSchemaVersion,
				Capabilities: []ExternalBuyerPaymentCapability{{
					Protocol: PaymentRailX402, X402Version: 2, Scheme: ExactScheme,
					Network: "eip155:8453", Asset: BaseSepoliaUSDCAsset,
				}},
			},
		},
		{
			name:    "local mock runtime",
			catalog: NewMockAdapter().PaymentCapabilities(),
			request: ExternalBuyerCompatibilityRequest{
				SchemaVersion: ExternalBuyerCapabilitiesSchemaVersion,
				Capabilities: []ExternalBuyerPaymentCapability{{
					Protocol: PaymentRailX402, X402Version: 2, Scheme: ExactScheme,
					Network: BaseSepoliaNetwork, Asset: BaseSepoliaUSDCAsset,
				}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := DetectExternalBuyerCompatibility(test.catalog, test.request)
			if result.Compatible || result.SelectedCapability != nil ||
				result.ReasonCode != ExternalBuyerNoCompatibleCapabilityReason {
				t.Fatalf("compatibility result = %#v", result)
			}
		})
	}
}
