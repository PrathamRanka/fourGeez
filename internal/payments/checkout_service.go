package payments

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/fourgeez/agentpay/internal/browserpurchase"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/observability"
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
	authorizer            CommerceAuthorizer
	browserPurchases      BrowserPurchaseLifecycle
	clock                 domain.Clock
}

// SetBrowserPurchaseLifecycle connects browser checkout to its durable grant.
func (service *CheckoutService) SetBrowserPurchaseLifecycle(lifecycle BrowserPurchaseLifecycle) {
	service.browserPurchases = lifecycle
}

// NewCheckoutServiceWithCommerceAuthorizer creates the launch checkout use case
// with fresh entitlement checks at every pre-finality payment boundary.
func NewCheckoutServiceWithCommerceAuthorizer(
	resolver *PaidRouteService,
	adapter Adapter,
	transactionRepository CheckoutTransactionRepository,
	evidenceRecorder PaymentEvidenceRecorder,
	executor PaidExecutor,
	clock domain.Clock,
	authorizer CommerceAuthorizer,
) *CheckoutService {
	service := NewCheckoutService(
		resolver,
		adapter,
		transactionRepository,
		evidenceRecorder,
		executor,
		clock,
	)
	service.authorizer = authorizer
	return service
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
	requestBodyHash, err := intents.HashRequestBody(request.Body, request.ContentType)
	if err != nil || requestBodyHash != resolved.PurchaseIntent.RequestBodyHash() {
		return CheckoutResult{}, ErrPaidRouteMismatch
	}
	if issues := catalog.ValidateJSONAgainstSchema(resolved.Route.InputSchema, request.Body, request.ContentType); len(issues) > 0 {
		return CheckoutResult{}, RequestValidationError{Issues: issues}
	}
	transaction, _, err := service.loadOrCreateTransaction(ctx, resolved)
	if err != nil {
		return CheckoutResult{}, err
	}
	if err := service.claimBrowserTransaction(ctx, transaction); err != nil {
		return CheckoutResult{}, err
	}
	if err := service.evidenceRecorder.RecordPaymentChallenge(
		ctx,
		transaction.TransactionID(),
		evidence.PaymentChallengeFacts{
			Amount:  resolved.Requirements.Amount.String(),
			Asset:   resolved.Requirements.Asset,
			Network: resolved.Requirements.Network,
		},
	); err != nil {
		observability.Record(observability.EventEvidenceFailure)
		return CheckoutResult{}, err
	}
	if strings.TrimSpace(request.PaymentProof) == "" {
		if transaction.Status() == transactions.StatusPaymentVerified &&
			transaction.PaymentFinality() == transactions.PaymentFinalityFinalized {
			return service.executeFinalized(ctx, request, resolved, transaction, "", "")
		}
		if err := service.authorizeCommerce(ctx, resolved.Seller.SellerID, CommerceOperationChallenge); err != nil {
			return CheckoutResult{}, err
		}
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

	proofHash, err := paymentProofHash(request.PaymentProof)
	if err != nil {
		return CheckoutResult{}, err
	}
	if transaction.PaymentIdentifier() != "" {
		if transaction.PaymentProofHash() != proofHash {
			return CheckoutResult{}, ErrPaymentReplay
		}
		if transaction.Status() != transactions.StatusPaymentVerified {
			return CheckoutResult{}, ErrPaymentReplay
		}
		if transaction.PaymentFinality() == transactions.PaymentFinalityFinalized {
			payerAddress := ""
			if transaction.PurchaseChannel() == transactions.PurchaseChannelBrowser && service.browserPurchases != nil {
				verification, verifyErr := service.adapter.Verify(ctx, request.PaymentProof, resolved.Requirements)
				if verifyErr != nil {
					return CheckoutResult{}, verifyErr
				}
				payerAddress = verification.PayerAddress
			}
			return service.executeFinalized(ctx, request, resolved, transaction, "", payerAddress)
		}
		if transaction.PaymentFinality() != transactions.PaymentFinalityConfirmed {
			return CheckoutResult{}, ErrPaymentReplay
		}
	}

	if err := service.authorizeCommerce(ctx, resolved.Seller.SellerID, CommerceOperationVerification); err != nil {
		return CheckoutResult{}, err
	}
	paymentIdentifier := transaction.PaymentIdentifier()
	payerAddress := ""
	if paymentIdentifier == "" {
		verification, verifyErr := service.adapter.Verify(
			ctx,
			request.PaymentProof,
			resolved.Requirements,
		)
		if verifyErr != nil {
			if IsRetryable(verifyErr) {
				observability.Record(observability.EventFacilitatorFailure)
			}
			if isTerminalPaymentRejection(verifyErr) {
				challenge, challengeErr := service.adapter.CreateChallenge(
					ctx,
					resolved.Requirements,
				)
				if challengeErr != nil {
					return CheckoutResult{}, challengeErr
				}
				return CheckoutResult{
					TransactionID:  transaction.TransactionID(),
					Challenge:      &challenge,
					RecoveryAction: RecoveryActionSignFreshAuthorization,
				}, verifyErr
			}
			if errors.Is(verifyErr, ErrPaymentCapabilityUnsupported) {
				return CheckoutResult{
					TransactionID:  transaction.TransactionID(),
					RecoveryAction: RecoveryActionSignFreshAuthorization,
				}, verifyErr
			}
			if IsRetryable(verifyErr) {
				return CheckoutResult{
					TransactionID:  transaction.TransactionID(),
					RecoveryAction: RecoveryActionRetrySameRequest,
				}, verifyErr
			}
			return CheckoutResult{}, verifyErr
		}
		paymentIdentifier = verification.PaymentIdentifier
		payerAddress = verification.PayerAddress
		expectedVersion := transaction.Version()
		if err := transaction.VerifyPayment(
			paymentIdentifier,
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
	}
	if err := service.evidenceRecorder.RecordPaymentVerification(
		ctx,
		transaction.TransactionID(),
		evidence.PaymentVerificationFacts{
			PaymentIdentifier: paymentIdentifier,
			PaymentProofHash:  proofHash,
		},
	); err != nil {
		observability.Record(observability.EventEvidenceFailure)
		return CheckoutResult{}, err
	}
	if err := service.authorizeCommerce(ctx, resolved.Seller.SellerID, CommerceOperationSettlement); err != nil {
		return CheckoutResult{}, err
	}
	settlement, err := service.adapter.Settle(
		ctx,
		request.PaymentProof,
		resolved.Requirements,
	)
	if err == nil && (!settlement.Settled || settlement.PaymentIdentifier != paymentIdentifier) {
		err = ErrPaymentRejected
	}
	if err == nil && payerAddress != "" && settlement.PayerAddress != "" &&
		!strings.EqualFold(payerAddress, settlement.PayerAddress) {
		err = ErrPaymentWalletMismatch
	}
	if err != nil {
		if IsRetryable(err) {
			observability.Record(observability.EventFacilitatorFailure)
		}
		if isTerminalPaymentRejection(err) {
			failedVersion := transaction.Version()
			if failErr := transaction.FailPayment(
				"settlement_rejected",
				"",
				domain.NewTimestamp(service.clock.Now()),
			); failErr != nil {
				return CheckoutResult{}, failErr
			}
			if updateErr := service.transactionRepository.Update(
				ctx,
				transaction,
				failedVersion,
			); updateErr != nil {
				return CheckoutResult{}, updateErr
			}
			return CheckoutResult{
				TransactionID:  transaction.TransactionID(),
				RecoveryAction: RecoveryActionStartNewCheckout,
			}, err
		}
		if IsRetryable(err) {
			return CheckoutResult{
				TransactionID:  transaction.TransactionID(),
				RecoveryAction: RecoveryActionRetrySamePayment,
			}, err
		}
		return CheckoutResult{}, err
	}
	finalizationVersion := transaction.Version()
	if err := transaction.FinalizePayment(
		paymentIdentifier,
		settlement.PaymentReference,
		domain.NewTimestamp(service.clock.Now()),
	); err != nil {
		return CheckoutResult{}, err
	}
	if err := service.transactionRepository.Update(
		ctx,
		transaction,
		finalizationVersion,
	); err != nil {
		if errors.Is(err, persistence.ErrConditionFailed) {
			return CheckoutResult{}, ErrPaymentReplay
		}
		return CheckoutResult{}, err
	}
	return service.executeFinalized(
		ctx,
		request,
		resolved,
		transaction,
		settlement.ResponseHeader,
		firstNonEmpty(payerAddress, settlement.PayerAddress),
	)
}

func isTerminalPaymentRejection(err error) bool {
	return errors.Is(err, ErrPaymentRejected) ||
		errors.Is(err, ErrPaymentAuthorizationExpired) ||
		errors.Is(err, ErrPaymentSignatureInvalid) ||
		errors.Is(err, ErrPaymentWalletMismatch) ||
		errors.Is(err, ErrPaymentFacilitatorRejected)
}

func (service *CheckoutService) executeFinalized(
	ctx context.Context,
	request CheckoutRequest,
	resolved ResolvedPaidRoute,
	transaction transactions.Transaction,
	settlementHeader string,
	payerAddress string,
) (CheckoutResult, error) {
	if err := service.completeBrowserPurchase(ctx, transaction, payerAddress); err != nil {
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
		SettlementHeader: settlementHeader,
	}, nil
}

func (service *CheckoutService) claimBrowserTransaction(ctx context.Context, transaction transactions.Transaction) error {
	if transaction.PurchaseChannel() != transactions.PurchaseChannelBrowser || service.browserPurchases == nil {
		return nil
	}
	purchaseSessionID, err := browserpurchase.ParsePurchaseSessionID(transaction.PurchaseSessionID())
	if err != nil {
		return err
	}
	_, err = service.browserPurchases.ClaimTransaction(ctx, purchaseSessionID, transaction.TransactionID())
	return err
}

func (service *CheckoutService) completeBrowserPurchase(ctx context.Context, transaction transactions.Transaction, payerAddress string) error {
	if transaction.PurchaseChannel() != transactions.PurchaseChannelBrowser || service.browserPurchases == nil || payerAddress == "" {
		return nil
	}
	purchaseSessionID, err := browserpurchase.ParsePurchaseSessionID(transaction.PurchaseSessionID())
	if err != nil {
		return err
	}
	return service.browserPurchases.Complete(
		ctx,
		purchaseSessionID,
		transaction.Network(),
		payerAddress,
		transaction.TransactionID(),
		transaction.UpdatedAt().Time(),
	)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func paymentProofHash(proof string) (intents.SHA256Digest, error) {
	digest := sha256.Sum256([]byte(proof))
	return intents.ParseSHA256Digest(hex.EncodeToString(digest[:]))
}

func (service *CheckoutService) authorizeCommerce(
	ctx context.Context,
	sellerID domain.ID,
	operation CommerceOperation,
) error {
	if service.authorizer == nil {
		// The original constructor is retained for local M7 migration fixtures.
		// Production wiring must use NewCheckoutServiceWithCommerceAuthorizer.
		return nil
	}
	return service.authorizer.AuthorizeCommerce(ctx, sellerID, operation)
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
			TransactionID:        transactionID,
			IntentID:             resolved.PurchaseIntent.IntentID(),
			SellerID:             resolved.Seller.SellerID,
			RouteID:              resolved.Route.RouteID,
			BuyerID:              resolved.PurchaseIntent.BuyerID(),
			ProductDisplayName:   resolved.PurchaseIntent.ProductDisplayName(),
			ProductSlug:          resolved.PurchaseIntent.ProductSlug(),
			PaymentDestinationID: resolved.PurchaseIntent.PaymentDestinationID(),
			PurchaseSessionID:    resolved.PurchaseIntent.PurchaseSessionID(),
			PurchaseChannel:      transactionPurchaseChannel(resolved.PurchaseIntent.PurchaseChannel()),
			PaymentRail:          transactions.PaymentRailX402,
			Amount:               resolved.PurchaseIntent.Amount(),
			Asset:                resolved.PurchaseIntent.Asset(),
			Network:              resolved.PurchaseIntent.Network(),
			CreatedAt:            now,
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

func transactionPurchaseChannel(channel intents.PurchaseChannel) transactions.PurchaseChannel {
	if channel == intents.PurchaseChannelBrowser {
		return transactions.PurchaseChannelBrowser
	}
	return transactions.PurchaseChannelAgent
}

// transactionIDForIntent preserves the intent ULID under the transaction prefix.
func transactionIDForIntent(intentID domain.ID) (domain.ID, error) {
	raw := strings.TrimPrefix(intentID.String(), string(domain.IntentIDPrefix))
	return domain.ParseID(
		string(domain.TransactionIDPrefix)+raw,
		domain.TransactionIDPrefix,
	)
}
