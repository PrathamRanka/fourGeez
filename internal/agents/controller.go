package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/fourgeez/agentpay/internal/approvals"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
)

// ErrUnknownTool identifies model requests outside the fixed buyer surface.
var ErrUnknownTool = errors.New("unknown buyer tool")

// Controller validates model calls before invoking authoritative domain services.
type Controller struct {
	catalogService  CatalogService
	intentService   IntentService
	approvalService ApprovalService
}

// NewController wires the model boundary to the HTTP-owned domain use cases.
func NewController(
	catalogService CatalogService,
	intentService IntentService,
	approvalService ApprovalService,
) *Controller {
	return &Controller{
		catalogService:  catalogService,
		intentService:   intentService,
		approvalService: approvalService,
	}
}

// Execute strictly decodes one untrusted tool call and invokes one use case.
func (controller *Controller) Execute(
	ctx context.Context,
	buyerID string,
	toolCall ToolCall,
) (ToolResult, error) {
	var value any
	var err error

	switch toolCall.Name {
	case "getStorefrontManifest":
		value, err = controller.getStorefrontManifest(ctx, toolCall.Input)
	case "createPurchaseIntent":
		value, err = controller.createPurchaseIntent(
			ctx,
			buyerID,
			toolCall.Input,
		)
	case "getPurchaseIntent":
		value, err = controller.getPurchaseIntent(ctx, toolCall.Input)
	case "createApprovalSession":
		value, err = controller.createApprovalSession(ctx, toolCall.Input)
	case "getApprovalSession":
		value, err = controller.getApprovalSession(ctx, toolCall.Input)
	default:
		return ToolResult{}, fmt.Errorf("%w: %s", ErrUnknownTool, toolCall.Name)
	}
	if err != nil {
		return ToolResult{}, err
	}

	return ToolResult{
		ToolCallID: toolCall.ID,
		Value:      value,
	}, nil
}

// getStorefrontManifest resolves routes server-side from the seller slug.
func (controller *Controller) getStorefrontManifest(
	ctx context.Context,
	input any,
) (any, error) {
	var request struct {
		Slug string `json:"slug"`
	}
	if err := decodeToolInput(input, &request); err != nil {
		return nil, err
	}

	return controller.catalogService.GetStorefrontManifest(ctx, request.Slug)
}

// createPurchaseIntent revalidates IDs, hashes, and the buyer ceiling in Go.
func (controller *Controller) createPurchaseIntent(
	ctx context.Context,
	buyerID string,
	input any,
) (any, error) {
	var request struct {
		RouteID         string `json:"routeId"`
		RequestBodyHash string `json:"requestBodyHash"`
		MaximumAmount   string `json:"maximumAmount"`
	}
	if err := decodeToolInput(input, &request); err != nil {
		return nil, err
	}

	routeID, err := domain.ParseID(request.RouteID, domain.RouteIDPrefix)
	if err != nil {
		return nil, err
	}
	requestBodyHash, err := intents.ParseSHA256Digest(request.RequestBodyHash)
	if err != nil {
		return nil, err
	}
	maximumAmount, err := domain.ParseAmount(request.MaximumAmount)
	if err != nil {
		return nil, err
	}

	purchaseIntent, err := controller.intentService.Create(
		ctx,
		buyerID,
		intents.CreateIntentRequest{
			RouteID:         routeID,
			RequestBodyHash: requestBodyHash,
			MaximumAmount:   maximumAmount,
		},
	)
	if err != nil {
		return nil, err
	}

	return purchaseIntentView(purchaseIntent), nil
}

// getPurchaseIntent reads the immutable server-side intent by validated ID.
func (controller *Controller) getPurchaseIntent(
	ctx context.Context,
	input any,
) (any, error) {
	var request struct {
		IntentID string `json:"intentId"`
	}
	if err := decodeToolInput(input, &request); err != nil {
		return nil, err
	}

	intentID, err := domain.ParseID(request.IntentID, domain.IntentIDPrefix)
	if err != nil {
		return nil, err
	}
	purchaseIntent, err := controller.intentService.Get(ctx, intentID)
	if err != nil {
		return nil, err
	}

	return purchaseIntentView(purchaseIntent), nil
}

// createApprovalSession binds approval to the persisted immutable intent.
func (controller *Controller) createApprovalSession(
	ctx context.Context,
	input any,
) (any, error) {
	var request struct {
		IntentID  string `json:"intentId"`
		Approvers []struct {
			Label string `json:"label"`
		} `json:"approvers"`
	}
	if err := decodeToolInput(input, &request); err != nil {
		return nil, err
	}

	intentID, err := domain.ParseID(request.IntentID, domain.IntentIDPrefix)
	if err != nil {
		return nil, err
	}
	approversRequest := make(
		[]approvals.ApproverRequest,
		len(request.Approvers),
	)
	for index, approver := range request.Approvers {
		approversRequest[index] = approvals.ApproverRequest{
			Label: approver.Label,
		}
	}

	approvalSession, err := controller.approvalService.Create(
		ctx,
		intentID,
		approvals.CreateSessionRequest{Approvers: approversRequest},
	)
	if err != nil {
		return nil, err
	}

	return approvalSessionView(approvalSession), nil
}

// getApprovalSession returns only the model-safe approval snapshot.
func (controller *Controller) getApprovalSession(
	ctx context.Context,
	input any,
) (any, error) {
	var request struct {
		SessionID string `json:"sessionId"`
	}
	if err := decodeToolInput(input, &request); err != nil {
		return nil, err
	}

	sessionID, err := domain.ParseID(request.SessionID, domain.ApprovalIDPrefix)
	if err != nil {
		return nil, err
	}
	approvalSession, err := controller.approvalService.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return approvalSessionView(approvalSession), nil
}

// decodeToolInput rejects fields omitted from the fixed model schema.
func decodeToolInput(input any, destination any) error {
	encodedInput, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("encode tool input: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(encodedInput))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode tool input: %w", err)
	}
	if err := ensureToolInputEOF(decoder); err != nil {
		return err
	}

	return nil
}

// ensureToolInputEOF prevents concatenated JSON values from bypassing validation.
func ensureToolInputEOF(decoder *json.Decoder) error {
	var trailingValue any
	if err := decoder.Decode(&trailingValue); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("tool input contains multiple JSON values")
		}
		return fmt.Errorf("decode trailing tool input: %w", err)
	}

	return nil
}

// purchaseIntentView exposes immutable fields without adding mutation capability.
func purchaseIntentView(purchaseIntent intents.PurchaseIntent) PurchaseIntentView {
	return PurchaseIntentView{
		IntentID:         purchaseIntent.IntentID(),
		SellerID:         purchaseIntent.SellerID(),
		RouteID:          purchaseIntent.RouteID(),
		RequestMethod:    purchaseIntent.RequestMethod(),
		RequestPath:      purchaseIntent.RequestPath(),
		RequestBodyHash:  purchaseIntent.RequestBodyHash(),
		Amount:           purchaseIntent.Amount(),
		Asset:            purchaseIntent.Asset(),
		Network:          purchaseIntent.Network(),
		MaximumAmount:    purchaseIntent.MaximumAmount(),
		RequiresApproval: purchaseIntent.RequiresApproval(),
		IntentHash:       purchaseIntent.IntentHash(),
		Status:           purchaseIntent.Status(),
		ExpiresAt:        purchaseIntent.ExpiresAt(),
	}
}

// approvalSessionView strips every token-bearing approval creation field.
func approvalSessionView(session approvals.SessionResponse) ApprovalSessionView {
	return ApprovalSessionView{
		SessionID:         session.SessionID,
		IntentID:          session.IntentID,
		IntentHash:        session.IntentHash,
		RequiredApprovals: session.RequiredApprovals,
		Decisions:         session.Decisions,
		Status:            session.Status,
		ExpiresAt:         session.ExpiresAt,
		UpdatedAt:         session.UpdatedAt,
	}
}
