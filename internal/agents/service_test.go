package agents

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestToolDefinitionsMatchBuyerAPIOperations verifies the fixed tool surface.
func TestToolDefinitionsMatchBuyerAPIOperations(t *testing.T) {
	t.Parallel()

	definitions := ToolDefinitions()
	wantNames := []string{
		"getStorefrontManifest",
		"createPurchaseIntent",
		"getPurchaseIntent",
		"createApprovalSession",
		"getApprovalSession",
	}
	if len(definitions) != len(wantNames) {
		t.Fatalf("tool count = %d", len(definitions))
	}
	for index, wantName := range wantNames {
		definition := definitions[index]
		if definition.Name != wantName {
			t.Fatalf("tool %d name = %q", index, definition.Name)
		}
		if definition.InputSchema.Type != "object" ||
			definition.InputSchema.AdditionalProperties {
			t.Fatalf("tool schema = %#v", definition.InputSchema)
		}
	}
	encoded, err := json.Marshal(definitions)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"privateKey",
		"walletSecret",
		"paymentSignature",
		"approvalToken",
	} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("tool schemas expose forbidden field %q", forbidden)
		}
	}
}

// TestCreateIntentToolUsesAtomicAmountString verifies money stays integral.
func TestCreateIntentToolUsesAtomicAmountString(t *testing.T) {
	t.Parallel()

	definitions := ToolDefinitions()
	schema := definitions[1].InputSchema
	maximumAmount := schema.Properties["maximumAmount"]
	if maximumAmount.Type != "string" || maximumAmount.Pattern != "^[0-9]+$" {
		t.Fatalf("maximumAmount schema = %#v", maximumAmount)
	}
}
