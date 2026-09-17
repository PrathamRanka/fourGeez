package intents

import "github.com/fourgeez/agentpay/internal/domain"

// Snapshot contains the complete persisted representation of a purchase intent.
type Snapshot struct {
	IntentID         domain.ID            `json:"intentId"`
	SellerID         domain.ID            `json:"sellerId"`
	RouteID          domain.ID            `json:"routeId"`
	BuyerID          string               `json:"buyerId"`
	RequestMethod    RequestMethod        `json:"requestMethod"`
	RequestPath      string               `json:"requestPath"`
	RequestBodyHash  SHA256Digest         `json:"requestBodyHash"`
	Amount           domain.Amount        `json:"amount"`
	Asset            string               `json:"asset"`
	Network          string               `json:"network"`
	MaximumAmount    domain.Amount        `json:"maximumAmount"`
	RequiresApproval bool                 `json:"requiresApproval"`
	IntentHash       SHA256Digest         `json:"intentHash"`
	Status           PurchaseIntentStatus `json:"status"`
	CreatedAt        domain.Timestamp     `json:"createdAt"`
	ExpiresAt        domain.Timestamp     `json:"expiresAt"`
}

// Snapshot returns the immutable purchase-intent persistence representation.
func (purchaseIntent PurchaseIntent) Snapshot() Snapshot {
	return Snapshot{
		IntentID:         purchaseIntent.intentID,
		SellerID:         purchaseIntent.sellerID,
		RouteID:          purchaseIntent.routeID,
		BuyerID:          purchaseIntent.buyerID,
		RequestMethod:    purchaseIntent.requestMethod,
		RequestPath:      purchaseIntent.requestPath,
		RequestBodyHash:  purchaseIntent.requestBodyHash,
		Amount:           purchaseIntent.amount,
		Asset:            purchaseIntent.asset,
		Network:          purchaseIntent.network,
		MaximumAmount:    purchaseIntent.maximumAmount,
		RequiresApproval: purchaseIntent.requiresApproval,
		IntentHash:       purchaseIntent.intentHash,
		Status:           purchaseIntent.status,
		CreatedAt:        purchaseIntent.createdAt,
		ExpiresAt:        purchaseIntent.expiresAt,
	}
}

// Restore validates and recreates an immutable purchase intent from storage.
func Restore(snapshot Snapshot) (PurchaseIntent, error) {
	restored, err := NewPurchaseIntent(PurchaseIntentParams{
		IntentID:         snapshot.IntentID,
		SellerID:         snapshot.SellerID,
		RouteID:          snapshot.RouteID,
		BuyerID:          snapshot.BuyerID,
		RequestMethod:    snapshot.RequestMethod,
		RequestPath:      snapshot.RequestPath,
		RequestBodyHash:  snapshot.RequestBodyHash,
		Amount:           snapshot.Amount,
		Asset:            snapshot.Asset,
		Network:          snapshot.Network,
		MaximumAmount:    snapshot.MaximumAmount,
		RequiresApproval: snapshot.RequiresApproval,
		CreatedAt:        snapshot.CreatedAt,
		ExpiresAt:        snapshot.ExpiresAt,
	})
	if err != nil {
		return PurchaseIntent{}, err
	}
	if restored.intentHash != snapshot.IntentHash || restored.status != snapshot.Status {
		return PurchaseIntent{}, domain.NewValidationError("intentHash", "persistence", "stored intent snapshot is inconsistent")
	}

	return restored, nil
}
