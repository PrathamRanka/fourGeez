package agents

import (
	"context"
	"errors"
	"fmt"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/payments"
)

var (
	// ErrRouteNotPublished prevents fallback execution of undiscovered routes.
	ErrRouteNotPublished = errors.New("route is not published by the storefront")
	// ErrUnexpectedToolResult identifies an invalid internal orchestration response.
	ErrUnexpectedToolResult = errors.New("unexpected tool result")
)

// DeterministicBuyer follows a fixed purchase sequence without model inference.
type DeterministicBuyer struct {
	toolExecutor    ToolExecutor
	checkoutService CheckoutService
}

// NewDeterministicBuyer creates the explicitly labeled Bedrock fallback.
func NewDeterministicBuyer(
	toolExecutor ToolExecutor,
	checkoutService CheckoutService,
) *DeterministicBuyer {
	return &DeterministicBuyer{
		toolExecutor:    toolExecutor,
		checkoutService: checkoutService,
	}
}

// Purchase discovers, validates, and advances one deterministic purchase.
func (buyer *DeterministicBuyer) Purchase(
	ctx context.Context,
	request DeterministicPurchaseRequest,
) (DeterministicPurchaseResult, error) {
	manifest, err := buyer.getManifest(ctx, request)
	if err != nil {
		return DeterministicPurchaseResult{}, err
	}
	selectedRoute, err := publishedRoute(manifest, request.RouteID.String())
	if err != nil {
		return DeterministicPurchaseResult{}, err
	}

	purchaseIntent, err := buyer.resolvePurchaseIntent(ctx, request)
	if err != nil {
		return DeterministicPurchaseResult{}, err
	}
	if purchaseIntent.RouteID != selectedRoute.RouteID {
		return DeterministicPurchaseResult{}, ErrRouteNotPublished
	}

	result := DeterministicPurchaseResult{
		Mode:           BuyerModeDeterministic,
		PurchaseIntent: purchaseIntent,
	}
	if purchaseIntent.RequiresApproval && request.ApprovalToken == "" {
		approvalSession, createErr := buyer.createApprovalSession(
			ctx,
			request,
			purchaseIntent,
		)
		if createErr != nil {
			return DeterministicPurchaseResult{}, createErr
		}
		result.ApprovalSession = &approvalSession
		result.AwaitingApproval = true
		return result, nil
	}

	checkout, err := buyer.checkoutService.Execute(
		ctx,
		payments.CheckoutRequest{
			PaidRouteRequest: payments.PaidRouteRequest{
				Slug:          request.Slug,
				Method:        catalog.RouteMethod(purchaseIntent.RequestMethod),
				ProxyPath:     purchaseIntent.RequestPath,
				IntentID:      purchaseIntent.IntentID,
				BuyerID:       request.BuyerID,
				ApprovalToken: request.ApprovalToken,
			},
			PaymentProof: request.PaymentProof,
			Body:         request.Body,
			ContentType:  request.ContentType,
		},
	)
	if err != nil {
		return DeterministicPurchaseResult{}, err
	}
	result.Checkout = checkout
	result.AwaitingPayment = checkout.Challenge != nil && checkout.Response == nil

	return result, nil
}

// getManifest uses the same validated storefront operation exposed to models.
func (buyer *DeterministicBuyer) getManifest(
	ctx context.Context,
	request DeterministicPurchaseRequest,
) (catalog.StorefrontManifest, error) {
	result, err := buyer.toolExecutor.Execute(
		ctx,
		request.BuyerID,
		ToolCall{
			Name: "getStorefrontManifest",
			Input: map[string]any{
				"slug": request.Slug,
			},
		},
	)
	if err != nil {
		return catalog.StorefrontManifest{}, err
	}
	manifest, ok := result.Value.(catalog.StorefrontManifest)
	if !ok {
		return catalog.StorefrontManifest{}, ErrUnexpectedToolResult
	}

	return manifest, nil
}

// resolvePurchaseIntent creates or reloads the immutable transaction proposal.
func (buyer *DeterministicBuyer) resolvePurchaseIntent(
	ctx context.Context,
	request DeterministicPurchaseRequest,
) (PurchaseIntentView, error) {
	toolCall := ToolCall{
		Name: "createPurchaseIntent",
		Input: map[string]any{
			"routeId":         request.RouteID.String(),
			"requestBodyHash": request.RequestBodyHash,
			"maximumAmount":   request.MaximumAmount,
		},
	}
	if request.ExistingIntentID != "" {
		toolCall = ToolCall{
			Name: "getPurchaseIntent",
			Input: map[string]any{
				"intentId": request.ExistingIntentID.String(),
			},
		}
	}

	result, err := buyer.toolExecutor.Execute(
		ctx,
		request.BuyerID,
		toolCall,
	)
	if err != nil {
		return PurchaseIntentView{}, err
	}
	purchaseIntent, ok := result.Value.(PurchaseIntentView)
	if !ok {
		return PurchaseIntentView{}, ErrUnexpectedToolResult
	}

	return purchaseIntent, nil
}

// createApprovalSession requests human approval and never decides it locally.
func (buyer *DeterministicBuyer) createApprovalSession(
	ctx context.Context,
	request DeterministicPurchaseRequest,
	purchaseIntent PurchaseIntentView,
) (ApprovalSessionView, error) {
	approvers := make([]map[string]any, len(request.ApproverLabels))
	for index, label := range request.ApproverLabels {
		approvers[index] = map[string]any{"label": label}
	}

	result, err := buyer.toolExecutor.Execute(
		ctx,
		request.BuyerID,
		ToolCall{
			Name: "createApprovalSession",
			Input: map[string]any{
				"intentId":  purchaseIntent.IntentID.String(),
				"approvers": approvers,
			},
		},
	)
	if err != nil {
		return ApprovalSessionView{}, err
	}
	approvalSession, ok := result.Value.(ApprovalSessionView)
	if !ok {
		return ApprovalSessionView{}, fmt.Errorf(
			"%w: approval session",
			ErrUnexpectedToolResult,
		)
	}

	return approvalSession, nil
}

// publishedRoute selects only the exact enabled route returned by discovery.
func publishedRoute(
	manifest catalog.StorefrontManifest,
	routeID string,
) (catalog.PaidRoute, error) {
	for _, paidRoute := range manifest.Routes {
		if paidRoute.RouteID.String() == routeID && paidRoute.Enabled {
			return paidRoute, nil
		}
	}

	return catalog.PaidRoute{}, ErrRouteNotPublished
}
