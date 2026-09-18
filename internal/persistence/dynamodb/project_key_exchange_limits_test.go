package dynamodb

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/domain"
)

func TestProjectKeyExchangeRateLimiterUsesConditionalCredentialWindow(t *testing.T) {
	t.Parallel()
	client := &fakeClient{}
	clock := domain.FixedClock{Value: time.Date(2026, time.September, 18, 10, 2, 30, 0, time.UTC)}
	limiter := NewProjectKeyExchangeRateLimiter(client, "agentpay-dev", clock, 10, time.Minute)
	credentialID := mustDynamoID(t, "key_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.CredentialIDPrefix)
	if err := limiter.AllowProjectKeyExchange(t.Context(), credentialID); err != nil {
		t.Fatal(err)
	}
	if client.updateInput == nil || readStringAttribute(client.updateInput.Key["PK"]) != credentialPartitionKey(credentialID.String()) ||
		readStringAttribute(client.updateInput.Key["SK"]) != projectKeyExchangeRateLimitSortKey("2026-09-18T10:02:00Z") ||
		client.updateInput.ConditionExpression == nil || !strings.Contains(*client.updateInput.ConditionExpression, "#count < :limit") {
		t.Fatalf("rate limit update = %#v", client.updateInput)
	}
	client.updateErr = &types.ConditionalCheckFailedException{Message: stringPointer("limited")}
	if err := limiter.AllowProjectKeyExchange(t.Context(), credentialID); !errors.Is(err, domain.ErrRateLimitExceeded) {
		t.Fatalf("limited error = %v", err)
	}
}
