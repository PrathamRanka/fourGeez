package dynamodb

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/approvals"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/transactions"
)

// TestDocumentedKeys verifies the persisted key contract.
func TestDocumentedKeys(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "seller", got: sellerPartitionKey("sel_123"), want: "SELLER#sel_123"},
		{name: "route", got: routeSortKey("rte_123"), want: "ROUTE#rte_123"},
		{name: "credential", got: credentialSortKey("key_123"), want: "CREDENTIAL#key_123"},
		{name: "intent", got: intentPartitionKey("int_123"), want: "INTENT#int_123"},
		{name: "approval", got: approvalPartitionKey("aps_123"), want: "APPROVAL#aps_123"},
		{name: "transaction", got: transactionPartitionKey("txn_123"), want: "TXN#txn_123"},
		{name: "event", got: eventSortKey(7), want: "EVENT#000007"},
		{name: "dispute", got: disputePartitionKey("dsp_123"), want: "DISPUTE#dsp_123"},
		{name: "idempotency", got: idempotencyPartitionKey("create-intent"), want: "IDEMPOTENCY#create-intent"},
		{name: "payment claim", got: paymentPartitionKey("pay-123"), want: "PAYMENT#pay-123"},
		{name: "slug claim", got: slugPartitionKey("demo-seller"), want: "SLUG#demo-seller"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("key = %q, want %q", test.got, test.want)
			}
		})
	}
}

// TestIntegrationCredentialRepositoryUsesSellerScopedKeys verifies no scan path.
func TestIntegrationCredentialRepositoryUsesSellerScopedKeys(t *testing.T) {
	t.Parallel()

	credential := testDynamoIntegrationCredential(t)
	client := &fakeClient{}
	repository := NewIntegrationCredentialRepository(client, "agentpay-dev")
	if err := repository.Create(t.Context(), credential); err != nil {
		t.Fatal(err)
	}
	if client.putInput == nil {
		t.Fatal("Create() did not write a credential")
	}
	if readStringAttribute(client.putInput.Item["PK"]) !=
		sellerPartitionKey(credential.SellerID().String()) ||
		readStringAttribute(client.putInput.Item["SK"]) !=
			credentialSortKey(credential.CredentialID().String()) {
		t.Fatalf("credential item = %#v", client.putInput.Item)
	}

	client.queryOutput = &awssdk.QueryOutput{
		Items: []map[string]types.AttributeValue{client.putInput.Item},
	}
	credentials, err := repository.ListBySeller(
		t.Context(),
		credential.SellerID(),
	)
	if err != nil || len(credentials) != 1 {
		t.Fatalf("ListBySeller() = (%v, %v)", credentials, err)
	}
	if client.queryInput == nil ||
		client.queryInput.KeyConditionExpression == nil ||
		!strings.Contains(*client.queryInput.KeyConditionExpression, "begins_with") {
		t.Fatalf("query input = %#v", client.queryInput)
	}
}

// testDynamoIntegrationCredential creates a valid credential fixture.
func testDynamoIntegrationCredential(t *testing.T) integrations.Credential {
	t.Helper()

	credential, err := integrations.NewCredential(
		integrations.CredentialParams{
			CredentialID: mustDynamoID(
				t,
				"key_01K5D09YJ0C0M7RJM4FWQ0K9H8",
				domain.CredentialIDPrefix,
			),
			SellerID: mustDynamoID(
				t,
				"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
				domain.SellerIDPrefix,
			),
			TokenHash: strings.Repeat("a", 64),
			Label:     "Codex",
			Scopes:    []integrations.Scope{integrations.ScopeRead},
			CreatedAt: testDynamoTime(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return credential
}

// TestTransactionCreateConditionallyClaimsPaymentIdentifier verifies replay protection.
func TestTransactionCreateConditionallyClaimsPaymentIdentifier(t *testing.T) {
	t.Parallel()
	client := &fakeClient{}
	repository := NewTransactionRepository(client, "agentpay-dev")
	transaction := testDynamoTransaction(t)
	if err := repository.Create(context.Background(), transaction); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	input := client.transactWriteInput
	if input == nil || len(input.TransactItems) != 2 {
		t.Fatalf("TransactItems = %v, want payment claim and transaction", input)
	}
	claim := input.TransactItems[0].Put
	if readStringAttribute(claim.Item["PK"]) != paymentPartitionKey(transaction.PaymentIdentifier()) ||
		claim.ConditionExpression == nil ||
		*claim.ConditionExpression != "attribute_not_exists(PK)" {
		t.Fatalf("payment claim = %#v", claim)
	}
	profile := input.TransactItems[1].Put
	if readStringAttribute(profile.Item["GSI2PK"]) != paymentPartitionKey(transaction.PaymentIdentifier()) ||
		readStringAttribute(profile.Item["GSI2SK"]) != "TXN#"+transaction.TransactionID().String() {
		t.Fatalf("transaction payment index is incorrect: %#v", profile.Item)
	}
}

// TestTransactionClaimForwardingMapsConditionalLoss verifies exactly-one claiming.
func TestTransactionClaimForwardingMapsConditionalLoss(t *testing.T) {
	t.Parallel()
	transaction := testDynamoTransaction(t)
	transactionItem, err := marshalTransactionItem(transaction)
	if err != nil {
		t.Fatalf("marshalTransactionItem() error = %v", err)
	}
	client := &fakeClient{
		getOutput: &awssdk.GetItemOutput{
			Item: transactionItem,
		},
		updateErr: &types.ConditionalCheckFailedException{
			Message: stringPointer("lost"),
		},
	}
	repository := NewTransactionRepository(client, "agentpay-dev")
	_, won, err := repository.ClaimForwarding(
		context.Background(),
		transaction.TransactionID(),
		transaction.Version(),
		transaction.UpdatedAt().Add(time.Second),
	)
	if err != nil || won {
		t.Fatalf("ClaimForwarding() = (%v, %v), want lost claim without error", won, err)
	}
	if client.updateInput == nil || client.updateInput.ConditionExpression == nil || *client.updateInput.ConditionExpression != "#status = :verified AND #version = :expected" {
		t.Fatalf("condition = %#v", client.updateInput)
	}
}

// TestEvidenceAppendConditionChecksPreviousHash verifies append-only chain writes.
func TestEvidenceAppendConditionChecksPreviousHash(t *testing.T) {
	t.Parallel()
	client := &fakeClient{}
	repository := NewEvidenceRepository(client, "agentpay-dev")
	events := testDynamoEvidenceChain(t)
	if err := repository.Append(context.Background(), events[1]); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	input := client.transactWriteInput
	if input == nil || len(input.TransactItems) != 2 {
		t.Fatalf("TransactItems = %#v", input)
	}
	check := input.TransactItems[0].ConditionCheck
	if readStringAttribute(check.Key["SK"]) != eventSortKey(1) ||
		check.ConditionExpression == nil ||
		*check.ConditionExpression != "eventHash = :previousHash" {
		t.Fatalf("previous event check = %#v", check)
	}
	put := input.TransactItems[1].Put
	if put.ConditionExpression == nil || *put.ConditionExpression != "attribute_not_exists(PK) AND attribute_not_exists(SK)" {
		t.Fatalf("append condition = %#v", put)
	}
}

// TestApprovalCreateStoresInvitationChildren verifies the documented item layout.
func TestApprovalCreateStoresInvitationChildren(t *testing.T) {
	t.Parallel()

	client := &fakeClient{}
	repository := NewApprovalRepository(client, "agentpay-dev")
	session := testDynamoApprovalSession(t)

	if err := repository.Create(context.Background(), session); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	input := client.transactWriteInput
	if input == nil || len(input.TransactItems) != 3 {
		t.Fatalf("TransactItems = %#v, want profile and two invitation items", input)
	}
	for _, transactItem := range input.TransactItems[1:] {
		sortKey := readStringAttribute(transactItem.Put.Item["SK"])
		if !strings.HasPrefix(sortKey, "INVITE#") {
			t.Fatalf("invitation sort key = %q", sortKey)
		}
	}
}

// TestIdempotencySaveMapsConditionalFailureToExisting verifies replay detection.
func TestIdempotencySaveMapsConditionalFailureToExisting(t *testing.T) {
	t.Parallel()
	client := &fakeClient{putErr: &types.ConditionalCheckFailedException{Message: stringPointer("exists")}}
	store := NewIdempotencyStore(client, "agentpay-dev")
	key, _ := domain.ParseIdempotencyKey("request-123")
	idempotencyRecord := domain.IdempotencyRecord{
		Scope:          "create-intent",
		Key:            key,
		RequestHash:    strings.Repeat("a", 64),
		ResponseStatus: 201,
		CreatedAt:      testDynamoTime(),
		ExpiresAt:      testDynamoTime().Add(time.Hour),
	}
	created, err := store.SaveIfAbsent(context.Background(), idempotencyRecord)
	if err != nil || created {
		t.Fatalf("SaveIfAbsent() = (%v, %v), want existing without error", created, err)
	}
}

// TestConditionalErrorsAreStable verifies stable domain-level repository errors.
func TestConditionalErrorsAreStable(t *testing.T) {
	t.Parallel()
	client := &fakeClient{transactErr: &types.TransactionCanceledException{Message: stringPointer("cancelled")}}
	repository := NewTransactionRepository(client, "agentpay-dev")
	if err := repository.Create(context.Background(), testDynamoTransaction(t)); !errors.Is(err, persistence.ErrPaymentIdentifierConflict) {
		t.Fatalf("Create() error = %v, want payment identifier conflict", err)
	}
}

// fakeClient captures DynamoDB calls without requiring an AWS account.
type fakeClient struct {
	putInput           *awssdk.PutItemInput
	putErr             error
	getInput           *awssdk.GetItemInput
	getOutput          *awssdk.GetItemOutput
	updateInput        *awssdk.UpdateItemInput
	updateErr          error
	queryInput         *awssdk.QueryInput
	queryOutput        *awssdk.QueryOutput
	transactWriteInput *awssdk.TransactWriteItemsInput
	transactErr        error
}

// PutItem records the request and returns the configured result.
func (client *fakeClient) PutItem(
	_ context.Context,
	input *awssdk.PutItemInput,
	_ ...func(*awssdk.Options),
) (*awssdk.PutItemOutput, error) {
	client.putInput = input
	return &awssdk.PutItemOutput{}, client.putErr
}

// GetItem records the request and returns the configured result.
func (client *fakeClient) GetItem(
	_ context.Context,
	input *awssdk.GetItemInput,
	_ ...func(*awssdk.Options),
) (*awssdk.GetItemOutput, error) {
	client.getInput = input
	if client.getOutput == nil {
		return &awssdk.GetItemOutput{}, nil
	}
	return client.getOutput, nil
}

// UpdateItem records the request and returns the configured result.
func (client *fakeClient) UpdateItem(
	_ context.Context,
	input *awssdk.UpdateItemInput,
	_ ...func(*awssdk.Options),
) (*awssdk.UpdateItemOutput, error) {
	client.updateInput = input
	return &awssdk.UpdateItemOutput{}, client.updateErr
}

// Query records the request and returns the configured result.
func (client *fakeClient) Query(
	_ context.Context,
	input *awssdk.QueryInput,
	_ ...func(*awssdk.Options),
) (*awssdk.QueryOutput, error) {
	client.queryInput = input
	if client.queryOutput == nil {
		return &awssdk.QueryOutput{}, nil
	}
	return client.queryOutput, nil
}

// TransactWriteItems records the request and returns the configured result.
func (client *fakeClient) TransactWriteItems(
	_ context.Context,
	input *awssdk.TransactWriteItemsInput,
	_ ...func(*awssdk.Options),
) (*awssdk.TransactWriteItemsOutput, error) {
	client.transactWriteInput = input
	return &awssdk.TransactWriteItemsOutput{}, client.transactErr
}

// testDynamoTransaction creates a payment-verified transaction fixture.
func testDynamoTransaction(t *testing.T) transactions.Transaction {
	t.Helper()
	transaction, err := transactions.NewTransaction(transactions.TransactionParams{
		TransactionID: mustDynamoID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix),
		IntentID:      mustDynamoID(t, "int_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.IntentIDPrefix),
		SellerID:      mustDynamoID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		RouteID:       mustDynamoID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		BuyerID:       "agent-123",
		Amount:        domain.MustParseAmount("35000000"),
		Asset:         "test-usdc",
		Network:       "test-network",
		CreatedAt:     testDynamoTime(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := transaction.RequirePayment(testDynamoTime().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	digest, _ := intents.ParseSHA256Digest(strings.Repeat("a", 64))
	if err := transaction.VerifyPayment("payment-123", digest, testDynamoTime().Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	return transaction
}

// testDynamoEvidenceChain creates a valid two-event evidence chain.
func testDynamoEvidenceChain(t *testing.T) []evidence.Event {
	t.Helper()
	signer := dynamoSigner{}
	transactionID := mustDynamoID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix)
	first, err := evidence.Append(
		context.Background(),
		evidence.EventParams{
			EventID:       mustDynamoID(t, "evt_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.EvidenceIDPrefix),
			TransactionID: transactionID,
			Sequence:      1,
			EventType:     evidence.EventIntentCreated,
			ActorType:     evidence.ActorBuyer,
			Payload:       map[string]any{},
			CreatedAt:     testDynamoTime(),
		},
		nil,
		signer,
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := evidence.Append(
		context.Background(),
		evidence.EventParams{
			EventID:       mustDynamoID(t, "evt_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.EvidenceIDPrefix),
			TransactionID: transactionID,
			Sequence:      2,
			EventType:     evidence.EventPaymentVerified,
			ActorType:     evidence.ActorSystem,
			Payload:       map[string]any{},
			CreatedAt:     testDynamoTime().Add(time.Second),
		},
		&first,
		signer,
	)
	if err != nil {
		t.Fatal(err)
	}
	return []evidence.Event{first, second}
}

// testDynamoApprovalSession creates a pending two-person approval session.
func testDynamoApprovalSession(t *testing.T) approvals.Session {
	t.Helper()

	intentHash, err := intents.ParseSHA256Digest(strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	tokenGenerator := &dynamoTokenGenerator{
		tokens: []string{
			strings.Repeat("c", 43),
			strings.Repeat("d", 43),
		},
	}
	session, _, err := approvals.NewSession(
		approvals.SessionParams{
			SessionID: mustDynamoID(
				t,
				"aps_01K5D09YJ0C0M7RJM4FWQ0K9H7",
				domain.ApprovalIDPrefix,
			),
			IntentID: mustDynamoID(
				t,
				"int_01K5D09YJ0C0M7RJM4FWQ0K9H7",
				domain.IntentIDPrefix,
			),
			IntentHash:     intentHash,
			ApproverLabels: []string{"Finance", "Security"},
			CreatedAt:      testDynamoTime(),
			ExpiresAt:      testDynamoTime().Add(10 * time.Minute),
		},
		tokenGenerator,
	)
	if err != nil {
		t.Fatal(err)
	}

	return session
}

// dynamoTokenGenerator returns deterministic invitation tokens.
type dynamoTokenGenerator struct {
	tokens []string
	next   int
}

// NewToken returns the next deterministic invitation token.
func (generator *dynamoTokenGenerator) NewToken() (string, error) {
	token := generator.tokens[generator.next]
	generator.next++
	return token, nil
}

// dynamoSigner provides deterministic evidence signatures for repository tests.
type dynamoSigner struct{}

// Sign returns a deterministic signature for repository tests.
func (dynamoSigner) Sign(_ context.Context, digest []byte) (evidence.Signature, error) {
	return evidence.Signature{
		KeyID: "key",
		Value: string(digest),
	}, nil
}

// Verify compares a deterministic signature for repository tests.
func (dynamoSigner) Verify(_ context.Context, _ string, digest []byte, signature string) (bool, error) {
	return string(digest) == signature, nil
}

// mustDynamoID parses an identifier fixture.
func mustDynamoID(t *testing.T, raw string, prefix domain.IDPrefix) domain.ID {
	t.Helper()
	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}

// testDynamoTime returns the fixed repository-test timestamp.
func testDynamoTime() domain.Timestamp {
	return domain.NewTimestamp(
		time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	)
}

// readStringAttribute returns a DynamoDB string attribute value.
func readStringAttribute(value types.AttributeValue) string {
	stringValue, ok := value.(*types.AttributeValueMemberS)
	if ok {
		return stringValue.Value
	}
	return ""
}
