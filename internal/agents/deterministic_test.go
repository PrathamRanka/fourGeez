package agents

import (
	"context"
	"errors"
	"testing"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/payments"
	"github.com/fourgeez/agentpay/internal/proxy"
)

// TestDeterministicBuyerCompletesCheckoutWithoutBedrock verifies fallback parity.
func TestDeterministicBuyerCompletesCheckoutWithoutBedrock(t *testing.T) {
	t.Parallel()

	routeID := domain.ID(testRouteID)
	toolExecutor := &deterministicToolExecutor{
		manifest: catalog.StorefrontManifest{
			Seller: catalog.StorefrontSeller{
				Name: "Demo seller",
				Slug: "demo-seller",
			},
			Routes: []catalog.PaidRoute{
				{
					RouteID:     routeID,
					Method:      catalog.RouteMethodPost,
					PathPattern: "/weather",
					Enabled:     true,
				},
			},
		},
		purchaseIntent: PurchaseIntentView{
			IntentID:      domain.ID(testIntentID),
			RouteID:       routeID,
			RequestMethod: intents.RequestMethodPost,
			RequestPath:   "/weather",
			Status:        intents.PurchaseIntentStatusReady,
		},
	}
	checkoutService := &deterministicCheckoutService{
		result: payments.CheckoutResult{
			TransactionID: domain.ID("txn_01ARZ3NDEKTSV4RRFFQ69G5FAW"),
			Response: &proxy.ForwardResponse{
				StatusCode: 200,
			},
		},
	}
	buyer := NewDeterministicBuyer(toolExecutor, checkoutService)

	result, err := buyer.Purchase(
		t.Context(),
		DeterministicPurchaseRequest{
			BuyerID:         "buyer-123",
			Slug:            "demo-seller",
			RouteID:         routeID,
			RequestBodyHash: testBodyHash,
			MaximumAmount:   "2500",
			PaymentProof:    payments.MockApprovedProof,
			Body:            []byte(`{"city":"Seattle"}`),
			ContentType:     "application/json",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Mode != BuyerModeDeterministic {
		t.Fatalf("mode = %q", result.Mode)
	}
	if result.Checkout.Response == nil || result.Checkout.Response.StatusCode != 200 {
		t.Fatalf("checkout = %#v", result.Checkout)
	}
	if checkoutService.request.IntentID != domain.ID(testIntentID) ||
		checkoutService.request.Method != catalog.RouteMethodPost ||
		checkoutService.request.ProxyPath != "/weather" {
		t.Fatalf("checkout request = %#v", checkoutService.request)
	}
}

// TestDeterministicBuyerNeverInfersApproval verifies protected routes stop safely.
func TestDeterministicBuyerNeverInfersApproval(t *testing.T) {
	t.Parallel()

	routeID := domain.ID(testRouteID)
	toolExecutor := &deterministicToolExecutor{
		manifest: catalog.StorefrontManifest{
			Routes: []catalog.PaidRoute{
				{
					RouteID: routeID,
					Enabled: true,
				},
			},
		},
		purchaseIntent: PurchaseIntentView{
			IntentID:         domain.ID(testIntentID),
			RouteID:          routeID,
			RequiresApproval: true,
			Status:           intents.PurchaseIntentStatusApprovalPending,
		},
		approvalSession: ApprovalSessionView{
			SessionID: domain.ID(testSessionID),
		},
	}
	checkoutService := &deterministicCheckoutService{}
	buyer := NewDeterministicBuyer(toolExecutor, checkoutService)

	result, err := buyer.Purchase(
		t.Context(),
		DeterministicPurchaseRequest{
			BuyerID:         "buyer-123",
			Slug:            "demo-seller",
			RouteID:         routeID,
			RequestBodyHash: testBodyHash,
			MaximumAmount:   "2500",
			ApproverLabels:  []string{"Finance", "Security"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.AwaitingApproval || result.ApprovalSession == nil {
		t.Fatalf("result = %#v", result)
	}
	if checkoutService.calls != 0 {
		t.Fatalf("checkout calls = %d", checkoutService.calls)
	}
}

// TestDeterministicBuyerContinuesApprovedIntent verifies the same intent executes.
func TestDeterministicBuyerContinuesApprovedIntent(t *testing.T) {
	t.Parallel()

	routeID := domain.ID(testRouteID)
	intentID := domain.ID(testIntentID)
	toolExecutor := &deterministicToolExecutor{
		manifest: catalog.StorefrontManifest{
			Routes: []catalog.PaidRoute{
				{
					RouteID: routeID,
					Enabled: true,
				},
			},
		},
		purchaseIntent: PurchaseIntentView{
			IntentID:         intentID,
			RouteID:          routeID,
			RequestMethod:    intents.RequestMethodGet,
			RequestPath:      "/research",
			RequiresApproval: true,
			Status:           intents.PurchaseIntentStatusApprovalPending,
		},
	}
	checkoutService := &deterministicCheckoutService{
		result: payments.CheckoutResult{
			TransactionID: domain.ID("txn_01ARZ3NDEKTSV4RRFFQ69G5FAW"),
			Response: &proxy.ForwardResponse{
				StatusCode: 200,
			},
		},
	}
	buyer := NewDeterministicBuyer(toolExecutor, checkoutService)

	result, err := buyer.Purchase(
		t.Context(),
		DeterministicPurchaseRequest{
			BuyerID:          "buyer-123",
			Slug:             "demo-seller",
			RouteID:          routeID,
			ExistingIntentID: intentID,
			ApprovalToken:    "opaque-token-for-payment-boundary",
			PaymentProof:     payments.MockApprovedProof,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.PurchaseIntent.IntentID != intentID {
		t.Fatalf("intent ID = %q", result.PurchaseIntent.IntentID)
	}
	if checkoutService.request.IntentID != intentID {
		t.Fatalf("checkout intent ID = %q", checkoutService.request.IntentID)
	}
	if len(toolExecutor.toolNames) != 2 ||
		toolExecutor.toolNames[1] != "getPurchaseIntent" {
		t.Fatalf("tool sequence = %#v", toolExecutor.toolNames)
	}
}

// TestDeterministicBuyerRejectsUnpublishedRoute verifies server discovery wins.
func TestDeterministicBuyerRejectsUnpublishedRoute(t *testing.T) {
	t.Parallel()

	buyer := NewDeterministicBuyer(
		&deterministicToolExecutor{
			manifest: catalog.StorefrontManifest{},
		},
		&deterministicCheckoutService{},
	)
	_, err := buyer.Purchase(
		t.Context(),
		DeterministicPurchaseRequest{
			BuyerID:         "buyer-123",
			Slug:            "demo-seller",
			RouteID:         domain.ID(testRouteID),
			RequestBodyHash: testBodyHash,
			MaximumAmount:   "2500",
		},
	)
	if !errors.Is(err, ErrRouteNotPublished) {
		t.Fatalf("Purchase() error = %v", err)
	}
}

type deterministicToolExecutor struct {
	manifest        catalog.StorefrontManifest
	purchaseIntent  PurchaseIntentView
	approvalSession ApprovalSessionView
	toolNames       []string
}

// Execute returns deterministic domain results for orchestration tests.
func (executor *deterministicToolExecutor) Execute(
	_ context.Context,
	_ string,
	toolCall ToolCall,
) (ToolResult, error) {
	executor.toolNames = append(executor.toolNames, toolCall.Name)

	switch toolCall.Name {
	case "getStorefrontManifest":
		return ToolResult{Value: executor.manifest}, nil
	case "createPurchaseIntent", "getPurchaseIntent":
		return ToolResult{Value: executor.purchaseIntent}, nil
	case "createApprovalSession":
		return ToolResult{Value: executor.approvalSession}, nil
	default:
		return ToolResult{}, ErrUnknownTool
	}
}

type deterministicCheckoutService struct {
	request payments.CheckoutRequest
	result  payments.CheckoutResult
	err     error
	calls   int
}

// Execute records the checkout request and returns the configured result.
func (service *deterministicCheckoutService) Execute(
	_ context.Context,
	request payments.CheckoutRequest,
) (payments.CheckoutResult, error) {
	service.calls++
	service.request = request
	return service.result, service.err
}
