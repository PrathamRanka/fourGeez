package disputes

import (
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

func TestClassifyDisputeRules(t *testing.T) {
	t.Parallel()

	trueValue := true
	falseValue := false
	expectedAmount := domain.MustParseAmount("35000000")
	wrongAmount := domain.MustParseAmount("36000000")

	tests := []struct {
		name       string
		reason     Reason
		facts      Facts
		wantStatus Status
		wantCode   string
	}{
		{name: "unauthorized confirmed", reason: ReasonUnauthorized, facts: Facts{AuthorizationValid: &falseValue}, wantStatus: StatusRefundRecommended, wantCode: CodeAuthorizationNotValid},
		{name: "unauthorized disproved", reason: ReasonUnauthorized, facts: Facts{AuthorizationValid: &trueValue}, wantStatus: StatusDenied, wantCode: CodeAuthorizationValid},
		{name: "duplicate confirmed", reason: ReasonDuplicate, facts: Facts{DuplicatePayment: &trueValue}, wantStatus: StatusRefundRecommended, wantCode: CodeDuplicatePaymentConfirmed},
		{name: "duplicate disproved", reason: ReasonDuplicate, facts: Facts{DuplicatePayment: &falseValue}, wantStatus: StatusDenied, wantCode: CodeDuplicatePaymentNotFound},
		{name: "wrong amount confirmed", reason: ReasonWrongAmount, facts: Facts{ExpectedAmount: &expectedAmount, PaidAmount: &wrongAmount}, wantStatus: StatusRefundRecommended, wantCode: CodeWrongAmountConfirmed},
		{name: "wrong amount disproved", reason: ReasonWrongAmount, facts: Facts{ExpectedAmount: &expectedAmount, PaidAmount: &expectedAmount}, wantStatus: StatusDenied, wantCode: CodeAmountMatchesIntent},
		{name: "not delivered confirmed", reason: ReasonNotDelivered, facts: Facts{DeliverySucceeded: &falseValue}, wantStatus: StatusRefundRecommended, wantCode: CodeDeliveryNotConfirmed},
		{name: "not delivered disproved", reason: ReasonNotDelivered, facts: Facts{DeliverySucceeded: &trueValue}, wantStatus: StatusDenied, wantCode: CodeDeliveryConfirmed},
		{name: "quality requires seller", reason: ReasonQualityOrOutput, facts: Facts{}, wantStatus: StatusSellerReview, wantCode: CodeQualityReviewRequired},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dispute, err := Classify(validDisputeParams(t, test.reason), test.facts)
			if err != nil {
				t.Fatalf("Classify() error = %v", err)
			}
			if dispute.Status != test.wantStatus || dispute.ClassificationCode != test.wantCode {
				t.Fatalf("classification = (%q, %q), want (%q, %q)", dispute.Status, dispute.ClassificationCode, test.wantStatus, test.wantCode)
			}
			if dispute.RuleVersion != RuleVersion {
				t.Fatalf("RuleVersion = %q, want %q", dispute.RuleVersion, RuleVersion)
			}
		})
	}
}

func TestClassifyDisputeUsesSellerReviewWhenEvidenceIsMissing(t *testing.T) {
	t.Parallel()

	for _, reason := range []Reason{ReasonUnauthorized, ReasonDuplicate, ReasonWrongAmount, ReasonNotDelivered} {
		t.Run(string(reason), func(t *testing.T) {
			t.Parallel()
			dispute, err := Classify(validDisputeParams(t, reason), Facts{})
			if err != nil {
				t.Fatalf("Classify() error = %v", err)
			}
			if dispute.Status != StatusSellerReview || dispute.ClassificationCode != CodeInsufficientEvidence {
				t.Fatalf("classification = (%q, %q), want seller review with insufficient evidence", dispute.Status, dispute.ClassificationCode)
			}
		})
	}
}

func TestClassifyDisputeValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*Params)
		wantField string
	}{
		{name: "dispute ID", mutate: func(params *Params) { params.DisputeID = params.TransactionID }, wantField: "disputeId"},
		{name: "transaction ID", mutate: func(params *Params) { params.TransactionID = params.DisputeID }, wantField: "transactionId"},
		{name: "reason", mutate: func(params *Params) { params.Reason = "other" }, wantField: "reason"},
		{name: "statement", mutate: func(params *Params) { params.Statement = string(make([]byte, 2001)) }, wantField: "statement"},
		{name: "created at", mutate: func(params *Params) { params.CreatedAt = domain.Timestamp{} }, wantField: "createdAt"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			params := validDisputeParams(t, ReasonNotDelivered)
			test.mutate(&params)
			_, err := Classify(params, Facts{})
			assertDisputeValidationField(t, err, test.wantField)
		})
	}
}

func validDisputeParams(t *testing.T, reason Reason) Params {
	t.Helper()
	return Params{
		DisputeID:     mustDisputeID(t, "dsp_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.DisputeIDPrefix),
		TransactionID: mustDisputeID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix),
		Reason:        reason,
		Statement:     "The seller did not return a response.",
		CreatedAt:     domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC)),
	}
}

func mustDisputeID(t *testing.T, raw string, prefix domain.IDPrefix) domain.ID {
	t.Helper()
	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatalf("ParseID(%q) error = %v", raw, err)
	}
	return identifier
}

func assertDisputeValidationField(t *testing.T, err error, field string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected validation error for %q", field)
	}
	var validationErrors domain.ValidationErrors
	if errors.As(err, &validationErrors) && validationErrors.HasField(field) {
		return
	}
	t.Fatalf("error = %#v, want validation error for %q", err, field)
}
