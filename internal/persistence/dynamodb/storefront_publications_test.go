package dynamodb

import (
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/storefront"
)

func TestStorefrontPublicationRevisionUsesConditionalMonotonicWrite(t *testing.T) {
	t.Parallel()
	sellerID := mustDynamoID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	state := storefront.PublicationState{SellerID: sellerID, Fingerprint: strings.Repeat("a", 64), PublicationRevision: 2, UpdatedAt: testDynamoTime(), Version: 2}
	client := &fakeClient{}
	repository := NewStorefrontPublicationRepository(client, "agentpay-dev")
	if err := repository.Put(t.Context(), state, 1); err != nil {
		t.Fatal(err)
	}
	input := client.putInput
	if input == nil || input.ConditionExpression == nil || *input.ConditionExpression != "#version = :expectedVersion AND #revision < :nextRevision" {
		t.Fatalf("publication condition = %#v", input)
	}
	client.putErr = &types.ConditionalCheckFailedException{Message: stringPointer("lost")}
	if err := repository.Put(t.Context(), state, 1); !errors.Is(err, persistence.ErrConditionFailed) {
		t.Fatalf("Put() error = %v, want condition failed", err)
	}
}
