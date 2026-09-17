package transactions

import (
	"context"
	"errors"
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
)

const purchaseReceiptSchemaVersion = "1"

// GetReceiptForBuyer returns a receipt only to its bound authenticated buyer.
func (service *Service) GetReceiptForBuyer(
	ctx context.Context,
	transactionID domain.ID,
	buyerSubject string,
) (PurchaseReceipt, error) {
	transaction, err := service.repository.Get(ctx, transactionID)
	if err != nil {
		return PurchaseReceipt{}, err
	}
	if strings.TrimSpace(buyerSubject) == "" || transaction.BuyerID() != buyerSubject {
		return PurchaseReceipt{}, ErrReceiptAccess
	}
	return service.purchaseReceipt(ctx, transaction)
}

// GetReceiptForSeller returns a receipt only to its transaction-owning seller.
func (service *Service) GetReceiptForSeller(
	ctx context.Context,
	transactionID domain.ID,
	ownerSubject string,
) (PurchaseReceipt, error) {
	transaction, err := service.repository.Get(ctx, transactionID)
	if err != nil {
		return PurchaseReceipt{}, err
	}
	if service.sellerRepository == nil {
		return PurchaseReceipt{}, ErrReceiptAccess
	}
	seller, err := service.sellerRepository.GetSeller(ctx, transaction.SellerID())
	if err != nil || seller.OwnerSubject != ownerSubject {
		return PurchaseReceipt{}, ErrReceiptAccess
	}
	return service.purchaseReceipt(ctx, transaction)
}

// purchaseReceipt builds a receipt only from finalized state and verified evidence.
func (service *Service) purchaseReceipt(
	ctx context.Context,
	transaction Transaction,
) (PurchaseReceipt, error) {
	if transaction.PaymentFinality() != PaymentFinalityFinalized ||
		strings.TrimSpace(transaction.PaymentReference()) == "" {
		return PurchaseReceipt{}, ErrReceiptUnavailable
	}
	events, err := service.evidenceRepository.ListByTransaction(
		ctx,
		transaction.TransactionID(),
	)
	if err != nil {
		return PurchaseReceipt{}, err
	}
	if err := evidence.VerifyChain(ctx, events, service.evidenceVerifier); err != nil {
		if errors.Is(err, evidence.ErrChainInvalid) {
			return PurchaseReceipt{}, ErrReceiptEvidenceInvalid
		}
		return PurchaseReceipt{}, err
	}
	rootEvent := events[0]
	headEvent := events[len(events)-1]
	return PurchaseReceipt{
		SchemaVersion: purchaseReceiptSchemaVersion,
		Transaction:   receiptTransaction(transaction),
		Evidence: ReceiptEvidence{
			Verified:      true,
			EventCount:    len(events),
			RootEventHash: rootEvent.EventHash,
			HeadEventHash: headEvent.EventHash,
			Events:        events,
		},
	}, nil
}

// receiptTransaction copies only safe public transaction fields into a receipt.
func receiptTransaction(transaction Transaction) ReceiptTransaction {
	return ReceiptTransaction{
		TransactionID:    transaction.TransactionID(),
		IntentID:         transaction.IntentID(),
		SellerID:         transaction.SellerID(),
		RouteID:          transaction.RouteID(),
		BuyerID:          transaction.BuyerID(),
		Status:           transaction.Status(),
		Amount:           transaction.Amount(),
		Asset:            transaction.Asset(),
		Network:          transaction.Network(),
		PaymentFinality:  transaction.PaymentFinality(),
		PaymentReference: transaction.PaymentReference(),
		ReconciledAt:     transaction.ReconciledAt(),
		UpstreamStatus:   transaction.UpstreamStatus(),
		ResponseHash:     transaction.ResponseHash(),
		FailureCode:      transaction.FailureCode(),
		CreatedAt:        transaction.CreatedAt(),
		UpdatedAt:        transaction.UpdatedAt(),
	}
}
