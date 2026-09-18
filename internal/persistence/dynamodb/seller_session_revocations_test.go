package dynamodb

import (
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func TestSellerSessionRevocationRepositoryUsesHashedSessionKeyAndExpiry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	client := &fakeClient{}
	repository := NewSellerSessionRevocationRepository(client, "agentpay-dev")
	if err := repository.Revoke(t.Context(), "hashed-session", expiresAt); err != nil {
		t.Fatal(err)
	}
	if client.putInput == nil ||
		readStringAttribute(client.putInput.Item["PK"]) != sellerSessionPartitionKey("hashed-session") ||
		readStringAttribute(client.putInput.Item["SK"]) != sellerSessionRevocationSortKey {
		t.Fatalf("put input = %#v", client.putInput)
	}

	client.getOutput = &awssdk.GetItemOutput{Item: client.putInput.Item}
	revoked, err := repository.IsRevoked(t.Context(), "hashed-session", now)
	if err != nil || !revoked {
		t.Fatalf("revoked = %t, error = %v", revoked, err)
	}
	revoked, err = repository.IsRevoked(t.Context(), "hashed-session", expiresAt)
	if err != nil || revoked {
		t.Fatalf("expired revoked = %t, error = %v", revoked, err)
	}
}
