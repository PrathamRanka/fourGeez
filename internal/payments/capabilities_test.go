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
