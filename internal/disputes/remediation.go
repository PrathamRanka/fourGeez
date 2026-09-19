package disputes

import (
	"context"
	"errors"
	"strings"
	"unicode"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/transactions"
)

const (
	maximumRefundReferenceLength     = 512
	maximumRecordedByLength          = 256
	RefundVerificationSellerReported = "seller_reported"
)

var (
	ErrRefundNotAllowed         = errors.New("manual refund record is not allowed")
	ErrRemediationConflict      = errors.New("manual remediation conflicts with the existing record")
	ErrRemediationStateConflict = errors.New("manual remediation requires a refund-recommended dispute")
	ErrRemediationAccess        = errors.New("manual remediation record was not found")
)

type RecordManualRefundRequest struct {
	DisputeID  domain.ID
	SellerID   domain.ID
	Amount     domain.Amount
	Asset      string
	Network    string
	Reference  string
	RecordedBy string
}

type ManualRefundRecord struct {
	DisputeID         domain.ID        `json:"disputeId"`
	TransactionID     domain.ID        `json:"transactionId"`
	SellerID          domain.ID        `json:"sellerId"`
	Amount            domain.Amount    `json:"amount"`
	Asset             string           `json:"asset"`
	Network           string           `json:"network"`
	Reference         string           `json:"reference"`
	RecordedBy        string           `json:"recordedBy"`
	RecordedAt        domain.Timestamp `json:"recordedAt"`
	VerificationState string           `json:"verificationState"`
}

type ManualRefundRecordRepository interface {
	SaveIfAbsent(context.Context, ManualRefundRecord) (ManualRefundRecord, bool, error)
	Get(context.Context, domain.ID) (ManualRefundRecord, error)
}

type ManualRemediationService struct {
	disputes     Repository
	transactions TransactionRepository
	refunds      ManualRefundRecordRepository
	clock        domain.Clock
	audit        audit.Recorder
}

func NewManualRemediationService(
	disputes Repository,
	transactions TransactionRepository,
	refunds ManualRefundRecordRepository,
	clock domain.Clock,
) *ManualRemediationService {
	if clock == nil {
		clock = domain.SystemClock{}
	}
	return &ManualRemediationService{
		disputes: disputes, transactions: transactions, refunds: refunds, clock: clock,
		audit: audit.NoopRecorder{},
	}
}

func (service *ManualRemediationService) SetAuditRecorder(recorder audit.Recorder) {
	if recorder != nil {
		service.audit = recorder
	}
}

func (service *ManualRemediationService) RecordManualRefund(
	ctx context.Context,
	request RecordManualRefundRequest,
) (ManualRefundRecord, error) {
	if service == nil || service.disputes == nil || service.transactions == nil || service.refunds == nil {
		return ManualRefundRecord{}, ErrRefundNotAllowed
	}
	dispute, err := service.disputes.Get(ctx, request.DisputeID)
	if err != nil {
		return ManualRefundRecord{}, err
	}
	transaction, err := service.transactions.Get(ctx, dispute.TransactionID)
	if err != nil {
		return ManualRefundRecord{}, err
	}
	if request.SellerID == "" || transaction.SellerID() != request.SellerID {
		return ManualRefundRecord{}, ErrRemediationAccess
	}
	if dispute.Status != StatusRefundRecommended {
		return ManualRefundRecord{}, ErrRemediationStateConflict
	}
	if transaction.PaymentFinality() != transactions.PaymentFinalityFinalized ||
		request.Amount.Compare(transaction.Amount()) != 0 ||
		strings.TrimSpace(request.Asset) != transaction.Asset() ||
		strings.TrimSpace(request.Network) != transaction.Network() {
		return ManualRefundRecord{}, ErrRefundNotAllowed
	}
	reference := strings.TrimSpace(request.Reference)
	recordedBy := strings.TrimSpace(request.RecordedBy)
	if reference == "" || len(reference) > maximumRefundReferenceLength ||
		recordedBy == "" || len(recordedBy) > maximumRecordedByLength ||
		strings.IndexFunc(reference, unicode.IsControl) >= 0 ||
		strings.IndexFunc(recordedBy, unicode.IsControl) >= 0 {
		return ManualRefundRecord{}, ErrRefundNotAllowed
	}
	record := ManualRefundRecord{
		DisputeID: dispute.DisputeID, TransactionID: transaction.TransactionID(),
		SellerID: transaction.SellerID(), Amount: transaction.Amount(), Asset: transaction.Asset(),
		Network: transaction.Network(), Reference: reference, RecordedBy: recordedBy,
		RecordedAt: domain.NewTimestamp(service.clock.Now()), VerificationState: RefundVerificationSellerReported,
	}
	stored, created, err := service.refunds.SaveIfAbsent(ctx, record)
	if err != nil {
		return ManualRefundRecord{}, err
	}
	if created {
		if err := service.audit.Record(ctx, audit.RecordRequest{
			SellerID: request.SellerID, ActorType: audit.ActorTypeSellerUser,
			ActorID: recordedBy, Action: audit.ActionManualRefundRecorded,
			TargetType: audit.TargetTypeDispute, TargetID: dispute.DisputeID.String(),
			Outcome:       audit.OutcomeSucceeded,
			ChangedFields: []string{"amount", "asset", "network", "reference"},
		}); err != nil {
			return ManualRefundRecord{}, err
		}
		return record, nil
	}
	if stored.VerificationState == "" {
		stored.VerificationState = RefundVerificationSellerReported
	}
	if stored.DisputeID == record.DisputeID && stored.TransactionID == record.TransactionID &&
		stored.SellerID == record.SellerID && stored.Amount.Compare(record.Amount) == 0 &&
		stored.Asset == record.Asset && stored.Network == record.Network &&
		stored.Reference == record.Reference && stored.RecordedBy == record.RecordedBy {
		return stored, nil
	}
	return ManualRefundRecord{}, ErrRemediationConflict
}

func (service *ManualRemediationService) GetManualRefund(ctx context.Context, disputeID, sellerID domain.ID) (ManualRefundRecord, error) {
	if service == nil || service.disputes == nil || service.transactions == nil || service.refunds == nil {
		return ManualRefundRecord{}, ErrRefundNotAllowed
	}
	dispute, err := service.disputes.Get(ctx, disputeID)
	if err != nil {
		return ManualRefundRecord{}, err
	}
	transaction, err := service.transactions.Get(ctx, dispute.TransactionID)
	if err != nil {
		return ManualRefundRecord{}, err
	}
	if sellerID == "" || transaction.SellerID() != sellerID {
		return ManualRefundRecord{}, ErrRemediationAccess
	}
	record, err := service.refunds.Get(ctx, disputeID)
	if err != nil {
		return ManualRefundRecord{}, err
	}
	if record.VerificationState == "" {
		record.VerificationState = RefundVerificationSellerReported
	}
	return record, nil
}
