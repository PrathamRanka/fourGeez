package payments

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/proxy"
	"github.com/fourgeez/agentpay/internal/transactions"
)

// CheckoutService coordinates challenge, verification, and seller execution.
type CheckoutService struct {
	resolver              *PaidRouteService
	adapter               Adapter
	transactionRepository CheckoutTransactionRepository
	evidenceRecorder      PaymentEvidenceRecorder
	executor              PaidExecutor
	clock                 domain.Clock
}

// NewCheckoutService creates the paid-route checkout use case.
func NewCheckoutService(
	resolver *PaidRouteService,
	adapter Adapter,
	transactionRepository CheckoutTransactionRepository,
	evidenceRecorder PaymentEvidenceRecorder,
	executor PaidExecutor,
	clock domain.Clock,
) *CheckoutService {
	return &CheckoutService{
		resolver:              resolver,
		adapter:               adapter,
		transactionRepository: transactionRepository,
		evidenceRecorder:      evidenceRecorder,
		executor:              executor,
		clock:                 clock,
	}
}

// Execute returns a challenge or completes one verified seller request.
func (service *CheckoutService) Execute(
	ctx context.Context,
	request CheckoutRequest,
) (CheckoutResult, error) {
	resolved, err := service.resolver.Resolve(ctx, request.PaidRouteRequest)
	if err != nil {
		return CheckoutResult{}, err
	}
	transaction, created, err := service.loadOrCreateTransaction(ctx, resolved)
	if err != nil {
		return CheckoutResult{}, err
	}
	if created {
		if err := service.evidenceRecorder.RecordPaymentChallenge(
			ctx,
			transaction.TransactionID(),
			evidence.PaymentChallengeFacts{
				Amount:  resolved.Requirements.Amount.String(),
				Asset:   resolved.Requirements.Asset,
				Network: resolved.Requirements.Network,
			},
		); err != nil {
			return CheckoutResult{}, err
		}
	}
	if strings.TrimSpace(request.PaymentProof) == "" {
		challenge, err := service.adapter.CreateChallenge(
			ctx,
			resolved.Requirements,
		)
		if err != nil {
			return CheckoutResult{}, err
		}
		return CheckoutResult{
			TransactionID: transaction.TransactionID(),
			Challenge:     &challenge,
		}, nil
	}

	verification, err := service.adapter.Verify(
		ctx,
		request.PaymentProof,
		resolved.Requirements,
	)
	if err != nil {
		if errors.Is(err, ErrPaymentRejected) {
			challenge, challengeErr := service.adapter.CreateChallenge(
				ctx,
				resolved.Requirements,
			)
			if challengeErr != nil {
				return CheckoutResult{}, challengeErr
			}
			return CheckoutResult{
				TransactionID: transaction.TransactionID(),
				Challenge:     &challenge,
			}, err
		}
		return CheckoutResult{}, err
	}
	proofDigest := sha256.Sum256([]byte(request.PaymentProof))
	proofHash, err := intents.ParseSHA256Digest(
		hex.EncodeToString(proofDigest[:]),
	)
	if err != nil {
		return CheckoutResult{}, err
	}
	expectedVersion := transaction.Version()
	if err := transaction.VerifyPayment(
		verification.PaymentIdentifier,
		proofHash,
		domain.NewTimestamp(service.clock.Now()),
	); err != nil {
		return CheckoutResult{}, err
	}
	if err := service.transactionRepository.Update(
		ctx,
		transaction,
		expectedVersion,
	); err != nil {
		if errors.Is(err, persistence.ErrPaymentIdentifierConflict) ||
			errors.Is(err, persistence.ErrConditionFailed) {
			return CheckoutResult{}, ErrPaymentReplay
		}
		return CheckoutResult{}, err
	}
	if err := service.evidenceRecorder.RecordPaymentVerification(
		ctx,
		transaction.TransactionID(),
		evidence.PaymentVerificationFacts{
			PaymentIdentifier: verification.PaymentIdentifier,
			PaymentProofHash:  proofHash,
		},
	); err != nil {
		return CheckoutResult{}, err
	}
	settlement, err := service.adapter.Settle(
		ctx,
		request.PaymentProof,
		resolved.Requirements,
	)
	if err != nil {
		return CheckoutResult{}, err
	}
	response, err := service.executor.Execute(
		ctx,
		proxy.ExecutionRequest{
			Transaction: transaction,
			Seller:      resolved.Seller,
			Route:       resolved.Route,
			Method:      request.Method,
			Path:        request.ProxyPath,
			Body:        request.Body,
			ContentType: request.ContentType,
		},
	)
	if err != nil {
		return CheckoutResult{}, err
	}
	return CheckoutResult{
		TransactionID:    transaction.TransactionID(),
		Response:         &response,
		SettlementHeader: settlement.ResponseHeader,
	}, nil
}

// loadOrCreateTransaction initializes one deterministic transaction per intent.
func (service *CheckoutService) loadOrCreateTransaction(
	ctx context.Context,
	resolved ResolvedPaidRoute,
) (transactions.Transaction, bool, error) {
	transactionID, err := transactionIDForIntent(
		resolved.PurchaseIntent.IntentID(),
	)
	if err != nil {
		return transactions.Transaction{}, false, err
	}
	transaction, err := service.transactionRepository.Get(ctx, transactionID)
	if err == nil {
		return transaction, false, nil
	}
	if !errors.Is(err, persistence.ErrNotFound) {
		return transactions.Transaction{}, false, err
	}
	now := domain.NewTimestamp(service.clock.Now())
	transaction, err = transactions.NewTransaction(
		transactions.TransactionParams{
			TransactionID: transactionID,
			IntentID:      resolved.PurchaseIntent.IntentID(),
			SellerID:      resolved.Seller.SellerID,
			RouteID:       resolved.Route.RouteID,
			BuyerID:       resolved.PurchaseIntent.BuyerID(),
			Amount:        resolved.PurchaseIntent.Amount(),
			Asset:         resolved.PurchaseIntent.Asset(),
			Network:       resolved.PurchaseIntent.Network(),
			CreatedAt:     now,
		},
	)
	if err != nil {
		return transactions.Transaction{}, false, err
	}
	if resolved.PurchaseIntent.RequiresApproval() {
		if err := transaction.RequireApproval(now); err != nil {
			return transactions.Transaction{}, false, err
		}
		if err := transaction.MarkApproved(now); err != nil {
			return transactions.Transaction{}, false, err
		}
	}
	if err := transaction.RequirePayment(now); err != nil {
		return transactions.Transaction{}, false, err
	}
	if err := service.transactionRepository.Create(ctx, transaction); err != nil {
		if errors.Is(err, persistence.ErrAlreadyExists) {
			loaded, loadErr := service.transactionRepository.Get(ctx, transactionID)
			return loaded, false, loadErr
		}
		return transactions.Transaction{}, false, err
	}
	return transaction, true, nil
}

// transactionIDForIntent preserves the intent ULID under the transaction prefix.
func transactionIDForIntent(intentID domain.ID) (domain.ID, error) {
	raw := strings.TrimPrefix(intentID.String(), string(domain.IntentIDPrefix))
	return domain.ParseID(
		string(domain.TransactionIDPrefix)+raw,
		domain.TransactionIDPrefix,
	)
}
