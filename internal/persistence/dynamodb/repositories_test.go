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
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/notifications"
	"github.com/fourgeez/agentpay/internal/operations"
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
		{name: "product slug claim", got: productSlugClaimSortKey("research-report"), want: "PRODUCT_SLUG#research-report"},
		{name: "credential", got: credentialSortKey("key_123"), want: "CREDENTIAL#key_123"},
		{name: "webhook delivery", got: webhookDeliverySortKey("whd_123"), want: "WEBHOOK_DELIVERY#whd_123"},
		{name: "webhook event claim", got: webhookEventClaimSortKey("whk_123", "evt_123"), want: "WEBHOOK_EVENT#whk_123#evt_123"},
		{name: "seller plan", got: sellerPlanSortKey, want: "BILLING_PLAN"},
		{name: "subscription reconciliation", got: subscriptionReconciliationSortKey("00000000000000000007"), want: "SUBSCRIPTION_RECONCILIATION#00000000000000000007"},
		{name: "quota counter", got: quotaCounterSortKey("2026-09", "api_request"), want: "QUOTA#2026-09#api_request"},
		{name: "quota claim", got: quotaClaimSortKey("2026-09", "webhook_delivery", "source"), want: "QUOTA_CLAIM#2026-09#webhook_delivery#41cf6794ba4200b839c53531555f0f3998df4cbb01a4d5cb0b94e3ca5e23947d"},
		{name: "usage meter", got: usageMeterSortKey(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC), "mtr_123"), want: "METER#2026-09-17T10:00:00Z#mtr_123"},
		{name: "usage source", got: usageMeterSourceSortKey("successful_transaction", "txn_123"), want: "METER_SOURCE#successful_transaction#txn_123"},
		{name: "payment destination", got: paymentDestinationSortKey("dst_123"), want: "DESTINATION#dst_123"},
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

func TestCatalogRepositoryAtomicallyClaimsSellerProductSlug(t *testing.T) {
	t.Parallel()

	seller, err := catalog.NewSeller(catalog.SellerParams{
		SellerID:        mustDynamoID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		OwnerSubject:    "owner-123",
		Slug:            "demo-seller",
		Name:            "Demo Seller",
		UpstreamBaseURL: "https://seller.example",
		CreatedAt:       testDynamoTime(),
	})
	if err != nil {
		t.Fatal(err)
	}
	sellerRecord, err := newStoredRecord(
		sellerPartitionKey(seller.SellerID.String()),
		profileSortKey,
		"seller",
		seller,
	)
	if err != nil {
		t.Fatal(err)
	}
	sellerItem, err := marshalStoredRecord(sellerRecord)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{getOutput: &awssdk.GetItemOutput{Item: sellerItem}}
	repository := NewCatalogRepository(client, "agentpay-dev")
	route, err := catalog.NewPaidRoute(catalog.PaidRouteParams{
		RouteID:                mustDynamoID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		SellerID:               seller.SellerID,
		DisplayName:            "Research Report",
		ProductSlug:            "research-report",
		Method:                 catalog.RouteMethodPost,
		PathPattern:            "/research",
		Description:            "Research",
		MIMEType:               "application/json",
		Amount:                 domain.MustParseAmount("35000000"),
		Asset:                  "test-usdc",
		Network:                "test-network",
		PayTo:                  "0x123",
		UpstreamTimeoutSeconds: 20,
		CreatedAt:              testDynamoTime(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateRoute(t.Context(), route); err != nil {
		t.Fatal(err)
	}
	transaction := client.transactWriteInput
	if transaction == nil || len(transaction.TransactItems) != 2 {
		t.Fatalf("route transaction = %#v", transaction)
	}
	claim := transaction.TransactItems[0].Put
	if readStringAttribute(claim.Item["PK"]) != sellerPartitionKey(seller.SellerID.String()) ||
		readStringAttribute(claim.Item["SK"]) != productSlugClaimSortKey(route.ProductSlug) ||
		claim.ConditionExpression == nil || *claim.ConditionExpression != createItemCondition {
		t.Fatalf("product slug claim = %#v", claim)
	}

	client.transactErr = &types.TransactionCanceledException{Message: stringPointer("duplicate")}
	if err := repository.CreateRoute(t.Context(), route); !errors.Is(err, persistence.ErrAlreadyExists) {
		t.Fatalf("duplicate product slug error = %v", err)
	}
}

func TestCatalogRepositoryClaimsAndResolvesOwnerSubject(t *testing.T) {
	t.Parallel()

	seller, err := catalog.NewSeller(catalog.SellerParams{
		SellerID:     mustDynamoID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		OwnerSubject: "owner-123", Slug: "owner-store", Name: "Owner Store",
		UpstreamBaseURL: "https://seller.example", CreatedAt: testDynamoTime(),
	})
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{}
	repository := NewCatalogRepository(client, "agentpay-dev")
	if err := repository.CreateSeller(t.Context(), seller); err != nil {
		t.Fatal(err)
	}
	transaction := client.transactWriteInput
	if transaction == nil || len(transaction.TransactItems) != 3 {
		t.Fatalf("seller transaction = %#v", transaction)
	}
	ownerClaim := transaction.TransactItems[1].Put
	if readStringAttribute(ownerClaim.Item["PK"]) != ownerSubjectPartitionKey("owner-123") ||
		readStringAttribute(ownerClaim.Item["SK"]) != ownerSubjectSellerSortKey {
		t.Fatalf("owner claim = %#v", ownerClaim)
	}

	client.getOutputs = []*awssdk.GetItemOutput{
		{Item: ownerClaim.Item},
		{Item: transaction.TransactItems[2].Put.Item},
	}
	resolved, err := repository.ResolveSellerByOwnerSubject(t.Context(), "owner-123")
	if err != nil || resolved.SellerID != seller.SellerID {
		t.Fatalf("resolved = %#v, error = %v", resolved, err)
	}
}

func TestCatalogRepositoryBackfillsLegacyProductIdentityAtomically(t *testing.T) {
	t.Parallel()

	legacyRoute := catalog.PaidRoute{
		RouteID:                mustDynamoID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		SellerID:               mustDynamoID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		Method:                 catalog.RouteMethodPost,
		PathPattern:            "/research",
		Description:            "Research Report",
		MIMEType:               "application/json",
		Amount:                 domain.MustParseAmount("35000000"),
		Asset:                  "test-usdc",
		Network:                "test-network",
		PayTo:                  "0x123",
		UpstreamTimeoutSeconds: 20,
		LifecycleStatus:        catalog.RouteLifecyclePublished,
		Enabled:                true,
		CreatedAt:              testDynamoTime(),
		UpdatedAt:              testDynamoTime(),
		Version:                1,
	}
	legacyRecord, err := newStoredRecord(
		sellerPartitionKey(legacyRoute.SellerID.String()),
		routeSortKey(legacyRoute.RouteID.String()),
		"paidRoute",
		legacyRoute,
	)
	if err != nil {
		t.Fatal(err)
	}
	legacyRecord.Version = legacyRoute.Version
	legacyItem, err := marshalStoredRecord(legacyRecord)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{queryOutput: &awssdk.QueryOutput{Items: []map[string]types.AttributeValue{legacyItem}}}
	repository := NewCatalogRepository(client, "agentpay-dev")
	updated := legacyRoute
	updated.DisplayName = "Research Report"
	updated.ProductSlug = "research-report-4fwq0k9h7"
	updated.Amount = domain.MustParseAmount("36000000")
	updated.Version = 2
	updated.UpdatedAt = testDynamoTime().Add(time.Minute)

	if err := repository.UpdateRoute(t.Context(), updated, 1); err != nil {
		t.Fatal(err)
	}
	transaction := client.transactWriteInput
	if transaction == nil || len(transaction.TransactItems) != 2 {
		t.Fatalf("legacy migration transaction = %#v", transaction)
	}
	claim := transaction.TransactItems[0].Put
	if readStringAttribute(claim.Item["SK"]) != productSlugClaimSortKey(updated.ProductSlug) {
		t.Fatalf("legacy product slug claim = %#v", claim.Item)
	}
}

// TestQuotaCounterRepositoryUsesConditionalAtomicWrites verifies persisted quota safety.
func TestQuotaCounterRepositoryUsesConditionalAtomicWrites(t *testing.T) {
	t.Parallel()

	client := &fakeClient{}
	repository := NewQuotaCounterRepository(client, "agentpay-dev")
	request := operations.CounterRequest{
		SellerID:    mustDynamoID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		QuotaName:   operations.QuotaAPIRequest,
		PeriodStart: domain.NewTimestamp(time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)),
		PeriodEnd:   domain.NewTimestamp(time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)),
		Limit:       10,
		UpdatedAt:   testDynamoTime(),
	}
	if err := repository.Increment(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if client.updateInput == nil ||
		readStringAttribute(client.updateInput.Key["SK"]) != quotaCounterSortKey("2026-09", "api_request") ||
		client.updateInput.ConditionExpression == nil ||
		!strings.Contains(*client.updateInput.ConditionExpression, "#count < :limit") {
		t.Fatalf("quota update = %#v", client.updateInput)
	}

	client.updateInput = nil
	if err := repository.IncrementUnique(t.Context(), request, "whk_1:evt_1"); err != nil {
		t.Fatal(err)
	}
	if client.transactWriteInput == nil || len(client.transactWriteInput.TransactItems) != 2 {
		t.Fatalf("quota transaction = %#v", client.transactWriteInput)
	}
}

// TestSellerEntitlementRepositoryAtomicallyWritesProjectionAndReconciliation verifies authoritative storage.
func TestSellerEntitlementRepositoryAtomicallyWritesProjectionAndReconciliation(t *testing.T) {
	t.Parallel()

	client := &fakeClient{}
	repository := NewSellerEntitlementRepository(client, "agentpay-dev")
	entitlement, reconciliation := testDynamoSellerEntitlement(t)
	if err := repository.Apply(t.Context(), entitlement, reconciliation, 0); err != nil {
		t.Fatal(err)
	}
	input := client.transactWriteInput
	if input == nil || len(input.TransactItems) != 2 {
		t.Fatalf("entitlement transaction = %#v", input)
	}
	history := input.TransactItems[0].Put
	projection := input.TransactItems[1].Put
	if readStringAttribute(history.Item["SK"]) != subscriptionReconciliationSortKey(entitlement.SourceRevision()) ||
		history.ConditionExpression == nil || *history.ConditionExpression != createItemCondition {
		t.Fatalf("reconciliation item = %#v", history)
	}
	if readStringAttribute(projection.Item["PK"]) != sellerPartitionKey(entitlement.SellerID().String()) ||
		readStringAttribute(projection.Item["SK"]) != sellerPlanSortKey ||
		projection.ConditionExpression == nil || *projection.ConditionExpression != createItemCondition {
		t.Fatalf("entitlement item = %#v", projection)
	}
}

// TestUsageMeterRepositoryAtomicallyClaimsSource verifies immutable metering.
func TestUsageMeterRepositoryAtomicallyClaimsSource(t *testing.T) {
	t.Parallel()

	client := &fakeClient{}
	repository := NewUsageMeterEventRepository(client, "agentpay-dev")
	event := testDynamoUsageMeterEvent(t)
	stored, created, err := repository.CreateIfAbsent(t.Context(), event)
	if err != nil || !created || stored.MeterEventID() != event.MeterEventID() {
		t.Fatalf("CreateIfAbsent() = (%#v, %v, %v)", stored, created, err)
	}
	input := client.transactWriteInput
	if input == nil || len(input.TransactItems) != 2 {
		t.Fatalf("TransactItems = %#v", input)
	}
	claimSortKey := readStringAttribute(input.TransactItems[0].Put.Item["SK"])
	eventSortKey := readStringAttribute(input.TransactItems[1].Put.Item["SK"])
	if claimSortKey != usageMeterSourceSortKey(
		string(event.MeterName()),
		event.SourceTransactionID().String(),
	) || eventSortKey != usageMeterSortKey(
		event.OccurredAt().Time(),
		event.MeterEventID().String(),
	) {
		t.Fatalf("usage keys = (%q, %q)", claimSortKey, eventSortKey)
	}
}

// TestWebhookDeliveryRepositoryClaimsSubscriptionEventIdentity verifies idempotent fan-out.
func TestWebhookDeliveryRepositoryClaimsSubscriptionEventIdentity(t *testing.T) {
	t.Parallel()

	client := &fakeClient{}
	repository := NewWebhookDeliveryRepository(client, "agentpay-dev")
	delivery := testDynamoWebhookDelivery(t)
	stored, inserted, err := repository.CreateIfAbsent(t.Context(), delivery)
	if err != nil || !inserted || stored.DeliveryID() != delivery.DeliveryID() {
		t.Fatalf("CreateIfAbsent() = (%#v, %v, %v)", stored, inserted, err)
	}
	input := client.transactWriteInput
	if input == nil || len(input.TransactItems) != 2 {
		t.Fatalf("TransactItems = %#v", input)
	}
	claim := input.TransactItems[0].Put
	if readStringAttribute(claim.Item["SK"]) != webhookEventClaimSortKey(
		delivery.SubscriptionID().String(),
		delivery.Event().EventID.String(),
	) {
		t.Fatalf("event claim = %#v", claim.Item)
	}
	record := input.TransactItems[1].Put
	if readStringAttribute(record.Item["SK"]) != webhookDeliverySortKey(delivery.DeliveryID().String()) {
		t.Fatalf("delivery record = %#v", record.Item)
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
	if client.transactWriteInput == nil || len(client.transactWriteInput.TransactItems) != 2 {
		t.Fatalf("Create() transaction = %#v", client.transactWriteInput)
	}
	lookup := client.transactWriteInput.TransactItems[0].Put
	credentialItem := client.transactWriteInput.TransactItems[1].Put
	if readStringAttribute(lookup.Item["PK"]) != credentialPartitionKey(credential.CredentialID().String()) ||
		readStringAttribute(lookup.Item["SK"]) != credentialLookupSortKey {
		t.Fatalf("credential lookup = %#v", lookup.Item)
	}
	if readStringAttribute(credentialItem.Item["PK"]) !=
		sellerPartitionKey(credential.SellerID().String()) ||
		readStringAttribute(credentialItem.Item["SK"]) !=
			credentialSortKey(credential.CredentialID().String()) {
		t.Fatalf("credential item = %#v", credentialItem.Item)
	}

	client.queryOutput = &awssdk.QueryOutput{
		Items: []map[string]types.AttributeValue{credentialItem.Item},
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

func TestIntegrationCredentialRepositoryResolvesPublicCredentialIDWithoutScan(t *testing.T) {
	t.Parallel()

	credential := testDynamoIntegrationCredential(t)
	repository := NewIntegrationCredentialRepository(&fakeClient{}, "agentpay-dev")
	lookupItem, err := repository.marshalCredentialLookup(credential)
	if err != nil {
		t.Fatal(err)
	}
	credentialItem, err := repository.marshalCredential(credential)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{getOutputs: []*awssdk.GetItemOutput{{Item: lookupItem}, {Item: credentialItem}}}
	repository = NewIntegrationCredentialRepository(client, "agentpay-dev")
	stored, err := repository.GetByID(t.Context(), credential.CredentialID())
	if err != nil || stored.SellerID() != credential.SellerID() || client.getCallCount != 2 {
		t.Fatalf("GetByID() = (%#v, %v), calls=%d", stored.Snapshot(), err, client.getCallCount)
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
			TokenHash:        strings.Repeat("a", 64),
			Label:            "Codex",
			Scopes:           []integrations.Scope{integrations.ScopeRead},
			EntitlementEpoch: 1,
			CreatedAt:        testDynamoTime(),
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
	putErrors          []error
	putCallCount       int
	getInput           *awssdk.GetItemInput
	getOutput          *awssdk.GetItemOutput
	getOutputs         []*awssdk.GetItemOutput
	getCallCount       int
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
	client.putCallCount++
	if len(client.putErrors) > 0 {
		err := client.putErrors[0]
		client.putErrors = client.putErrors[1:]
		return &awssdk.PutItemOutput{}, err
	}
	return &awssdk.PutItemOutput{}, client.putErr
}

// GetItem records the request and returns the configured result.
func (client *fakeClient) GetItem(
	_ context.Context,
	input *awssdk.GetItemInput,
	_ ...func(*awssdk.Options),
) (*awssdk.GetItemOutput, error) {
	client.getInput = input
	client.getCallCount++
	if len(client.getOutputs) > 0 {
		output := client.getOutputs[0]
		client.getOutputs = client.getOutputs[1:]
		return output, nil
	}
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

// testDynamoWebhookDelivery creates one valid delivery fixture.
func testDynamoWebhookDelivery(t *testing.T) notifications.Delivery {
	t.Helper()
	createdAt := testDynamoTime()
	delivery, err := notifications.NewDelivery(notifications.DeliveryParams{
		DeliveryID:     mustDynamoID(t, "whd_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.WebhookDeliveryIDPrefix),
		SellerID:       mustDynamoID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		SubscriptionID: mustDynamoID(t, "whk_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.WebhookSubscriptionIDPrefix),
		Event: notifications.WebhookEvent{
			SchemaVersion: "1",
			EventID:       mustDynamoID(t, "evt_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.EvidenceIDPrefix),
			SellerID:      mustDynamoID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
			EventType:     notifications.EventPaymentVerified,
			OccurredAt:    createdAt,
			Payload:       map[string]any{"transactionId": "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7"},
		},
		PayloadHash: strings.Repeat("f", 64),
		CreatedAt:   createdAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	return delivery
}

// testDynamoSellerEntitlement creates one valid entitlement and history fixture.
func testDynamoSellerEntitlement(t *testing.T) (billing.SellerEntitlement, billing.EntitlementReconciliation) {
	t.Helper()
	entitlement, reconciliation, err := billing.ReconcileSellerEntitlement(nil, billing.EntitlementCandidate{
		SellerID: mustDynamoID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		PlanID:   billing.PlanStarter, PlanVersion: 1, Status: billing.EntitlementStatusActive,
		BillingPeriodStart: domain.NewTimestamp(time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)),
		BillingPeriodEnd:   domain.NewTimestamp(time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)),
		AccessEndsAt:       domain.NewTimestamp(time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)),
		Source:             billing.EntitlementSourceBillingProvider, Provider: billing.EntitlementProviderStripe,
		ProviderCustomerID: "cus_123", ProviderSubscriptionID: "sub_123", ProviderPriceID: "price_123", LastProviderEventID: "evt_123",
	}, testDynamoTime())
	if err != nil {
		t.Fatal(err)
	}
	return entitlement, reconciliation
}

func TestIntegrationCredentialRepositoryRotatesAtomically(t *testing.T) {
	t.Parallel()

	predecessor := testDynamoIntegrationCredential(t)
	successorID := mustDynamoID(t, "key_01K5D09YJ0C0M7RJM4FWQ0K9H9", domain.CredentialIDPrefix)
	successor, err := integrations.NewCredential(integrations.CredentialParams{
		CredentialID: successorID, SellerID: predecessor.SellerID(), TokenHash: strings.Repeat("b", 64),
		Label: predecessor.Label(), Scopes: predecessor.Scopes(), EntitlementEpoch: predecessor.EntitlementEpoch(), CreatedAt: testDynamoTime().Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := predecessor.Replace(successorID, testDynamoTime().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{}
	repository := NewIntegrationCredentialRepository(client, "agentpay-dev")
	replayKey, _ := domain.ParseIdempotencyKey("credential-rotate-1")
	replay := integrations.RotationReplay{
		Scope: "owner:rotate", Key: replayKey, RequestHash: strings.Repeat("c", 64),
		ProtectedResponse: []byte("protected"), SuccessorCredentialID: successorID,
		CreatedAt: testDynamoTime(), ExpiresAt: testDynamoTime().Add(10 * time.Minute),
	}
	if err := repository.Rotate(t.Context(), predecessor, successor, 1, replay); err != nil {
		t.Fatal(err)
	}
	input := client.transactWriteInput
	if input == nil || len(input.TransactItems) != 4 {
		t.Fatalf("rotation transaction = %#v", input)
	}
	if input.TransactItems[0].Put.ConditionExpression == nil || *input.TransactItems[0].Put.ConditionExpression != "#version = :expectedVersion" {
		t.Fatalf("predecessor condition = %#v", input.TransactItems[0].Put)
	}
}

// testDynamoUsageMeterEvent creates one immutable usage fixture.
func testDynamoUsageMeterEvent(t *testing.T) billing.UsageMeterEvent {
	t.Helper()
	event, err := billing.NewUsageMeterEvent(billing.UsageMeterEventParams{
		MeterEventID:        mustDynamoID(t, "mtr_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.UsageMeterEventIDPrefix),
		SellerID:            mustDynamoID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		MeterName:           billing.MeterSuccessfulTransaction,
		Quantity:            1,
		SourceTransactionID: mustDynamoID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix),
		PlanID:              billing.PlanStarter,
		PlanVersion:         1,
		OccurredAt:          testDynamoTime(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return event
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
