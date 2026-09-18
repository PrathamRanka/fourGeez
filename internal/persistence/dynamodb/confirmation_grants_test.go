package dynamodb

import (
	"testing"

	"github.com/fourgeez/agentpay/internal/authorization"
	"github.com/fourgeez/agentpay/internal/domain"
)

func TestConfirmationGrantRepositoryCreatesGrantAndBindingAtomically(t *testing.T) {
	t.Parallel()
	client := &fakeClient{}
	repository := NewConfirmationGrantRepository(client, "agentpay-dev")
	grant := authorization.ConfirmationGrant{
		ConfirmationGrantID: domain.ID("mcg_01K5D09YJ0C0M7RJM4FWQ0K9HA"),
		SellerID:            domain.ID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"),
		CredentialID:        domain.ID("key_01K5D09YJ0C0M7RJM4FWQ0K9H8"),
		Tool:                authorization.ToolConfigureStorefront, TargetType: authorization.ConfirmationTargetSeller,
		TargetID: domain.ID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"), ArgumentsSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExpectedResourceVersion: 1, TokenDigest: "digest", BindingHash: "binding",
		IssuedAt: testDynamoTime(), ExpiresAt: testDynamoTime().Add(5), Version: 1,
	}
	if err := repository.CreateReplacing(t.Context(), grant); err != nil {
		t.Fatal(err)
	}
	if client.transactWriteInput == nil || len(client.transactWriteInput.TransactItems) != 2 {
		t.Fatalf("transaction = %#v", client.transactWriteInput)
	}
	grantItem := client.transactWriteInput.TransactItems[0].Put.Item
	if readStringAttribute(grantItem["PK"]) != "MCP_CONFIRMATION#"+grant.ConfirmationGrantID.String() || readStringAttribute(grantItem["SK"]) != "PROFILE" {
		t.Fatalf("grant key = %#v", grantItem)
	}
	bindingItem := client.transactWriteInput.TransactItems[1].Put.Item
	if readStringAttribute(bindingItem["confirmationGrantId"]) != grant.ConfirmationGrantID.String() {
		t.Fatalf("binding confirmationGrantId = %#v", bindingItem["confirmationGrantId"])
	}
}

func TestConfirmationGrantRepositoryConsumesOnlyCurrentBinding(t *testing.T) {
	t.Parallel()
	client := &fakeClient{}
	repository := NewConfirmationGrantRepository(client, "agentpay-dev")
	grant := authorization.ConfirmationGrant{
		ConfirmationGrantID: domain.ID("mcg_01K5D09YJ0C0M7RJM4FWQ0K9HA"),
		SellerID:            domain.ID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"),
		BindingHash:         "binding",
		Version:             2,
	}

	if err := repository.Consume(t.Context(), grant, 1); err != nil {
		t.Fatal(err)
	}
	transaction := client.transactWriteInput
	if transaction == nil || len(transaction.TransactItems) != 2 {
		t.Fatalf("transaction = %#v", transaction)
	}
	check := transaction.TransactItems[0].ConditionCheck
	if check == nil || check.ConditionExpression == nil || *check.ConditionExpression != "confirmationGrantId = :confirmationGrantId" {
		t.Fatalf("binding condition = %#v", check)
	}
}
