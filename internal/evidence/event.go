package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/gowebpki/jcs"
)

const evidenceHashDomain = "agentpay.evidence.v1"

var ErrChainInvalid = errors.New("evidence chain is invalid")

type EventType string

const (
	EventIntentCreated     EventType = "intent.created"
	EventApprovalRequested EventType = "approval.requested"
	EventApprovalDecided   EventType = "approval.decided"
	EventApprovalResolved  EventType = "approval.resolved"
	EventPaymentChallenged EventType = "payment.challenged"
	EventPaymentVerified   EventType = "payment.verified"
	EventProxyForwarded    EventType = "proxy.forwarded"
	EventDeliverySucceeded EventType = "delivery.succeeded"
	EventDeliveryFailed    EventType = "delivery.failed"
	EventDisputeOpened     EventType = "dispute.opened"
	EventDisputeClassified EventType = "dispute.classified"
	EventDisputeResolved   EventType = "dispute.resolved"
)

type ActorType string

const (
	ActorSystem   ActorType = "system"
	ActorSeller   ActorType = "seller"
	ActorBuyer    ActorType = "buyer"
	ActorApprover ActorType = "approver"
)

type Signature struct {
	KeyID string
	Value string
}

// Signer is the external signing boundary implemented by KMS in production.
type Signer interface {
	Sign(ctx context.Context, digest []byte) (Signature, error)
	Verify(ctx context.Context, keyID string, digest []byte, signature string) (bool, error)
}

type EventParams struct {
	EventID       domain.ID
	TransactionID domain.ID
	Sequence      uint64
	EventType     EventType
	ActorType     ActorType
	ActorID       string
	Payload       map[string]any
	CreatedAt     domain.Timestamp
}

type Event struct {
	EventID           domain.ID             `json:"eventId"`
	TransactionID     domain.ID             `json:"transactionId"`
	Sequence          uint64                `json:"sequence"`
	EventType         EventType             `json:"eventType"`
	ActorType         ActorType             `json:"actorType"`
	ActorID           string                `json:"actorId,omitempty"`
	Payload           map[string]any        `json:"payload"`
	PreviousEventHash *intents.SHA256Digest `json:"previousEventHash"`
	EventHash         intents.SHA256Digest  `json:"eventHash"`
	KMSKeyID          string                `json:"kmsKeyId"`
	KMSSignature      string                `json:"kmsSignature"`
	CreatedAt         domain.Timestamp      `json:"createdAt"`
}

type hashPayload struct {
	SchemaVersion     string         `json:"schemaVersion"`
	EventID           string         `json:"eventId"`
	TransactionID     string         `json:"transactionId"`
	Sequence          uint64         `json:"sequence"`
	EventType         EventType      `json:"eventType"`
	ActorType         ActorType      `json:"actorType"`
	ActorID           string         `json:"actorId,omitempty"`
	Payload           map[string]any `json:"payload"`
	PreviousEventHash *string        `json:"previousEventHash"`
	CreatedAt         string         `json:"createdAt"`
}

func Append(ctx context.Context, params EventParams, previous *Event, signer Signer) (Event, error) {
	if err := validateEventParams(params, previous, signer); err != nil {
		return Event{}, err
	}

	event := Event{
		EventID:       params.EventID,
		TransactionID: params.TransactionID,
		Sequence:      params.Sequence,
		EventType:     params.EventType,
		ActorType:     params.ActorType,
		ActorID:       strings.TrimSpace(params.ActorID),
		Payload:       clonePayload(params.Payload),
		CreatedAt:     params.CreatedAt,
	}
	if previous != nil {
		previousHash := previous.EventHash
		event.PreviousEventHash = &previousHash
	}

	digest, digestBytes, err := calculateHash(event)
	if err != nil {
		return Event{}, err
	}
	signature, err := signer.Sign(ctx, digestBytes)
	if err != nil {
		return Event{}, err
	}
	if strings.TrimSpace(signature.KeyID) == "" || strings.TrimSpace(signature.Value) == "" {
		return Event{}, domain.NewValidationError("signature", "required", "signer must return a key identifier and signature")
	}
	event.EventHash = digest
	event.KMSKeyID = strings.TrimSpace(signature.KeyID)
	event.KMSSignature = strings.TrimSpace(signature.Value)
	return event, nil
}

func VerifyChain(ctx context.Context, events []Event, verifier Signer) error {
	if len(events) == 0 || verifier == nil {
		return ErrChainInvalid
	}
	for index, event := range events {
		if err := validateStoredEvent(event, index, events); err != nil {
			return fmt.Errorf("%w: event %d: %v", ErrChainInvalid, index+1, err)
		}
		digest, digestBytes, err := calculateHash(event)
		if err != nil || digest != event.EventHash {
			return fmt.Errorf("%w: event %d hash mismatch", ErrChainInvalid, index+1)
		}
		valid, err := verifier.Verify(ctx, event.KMSKeyID, digestBytes, event.KMSSignature)
		if err != nil {
			return err
		}
		if !valid {
			return fmt.Errorf("%w: event %d signature mismatch", ErrChainInvalid, index+1)
		}
	}
	return nil
}

func calculateHash(event Event) (intents.SHA256Digest, []byte, error) {
	var previousHash *string
	if event.PreviousEventHash != nil {
		value := event.PreviousEventHash.String()
		previousHash = &value
	}
	encoded, err := json.Marshal(hashPayload{
		SchemaVersion:     "1",
		EventID:           event.EventID.String(),
		TransactionID:     event.TransactionID.String(),
		Sequence:          event.Sequence,
		EventType:         event.EventType,
		ActorType:         event.ActorType,
		ActorID:           event.ActorID,
		Payload:           event.Payload,
		PreviousEventHash: previousHash,
		CreatedAt:         event.CreatedAt.String(),
	})
	if err != nil {
		return "", nil, err
	}
	canonical, err := jcs.Transform(encoded)
	if err != nil {
		return "", nil, domain.NewValidationError("payload", "canonical", "must contain canonicalizable JSON values")
	}
	hashInput := append(append([]byte(evidenceHashDomain), 0), canonical...)
	sum := sha256.Sum256(hashInput)
	digest, err := intents.ParseSHA256Digest(hex.EncodeToString(sum[:]))
	if err != nil {
		return "", nil, err
	}
	return digest, sum[:], nil
}

func validateEventParams(params EventParams, previous *Event, signer Signer) error {
	var validationErrors domain.ValidationErrors
	if _, err := domain.ParseID(params.EventID.String(), domain.EvidenceIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("eventId", "format", "must be an evidence-event identifier"))
	}
	if _, err := domain.ParseID(params.TransactionID.String(), domain.TransactionIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("transactionId", "format", "must be a transaction identifier"))
	}
	if !validEventType(params.EventType) {
		validationErrors = append(validationErrors, domain.NewValidationError("eventType", "supported", "must use the documented event vocabulary"))
	}
	if !validActorType(params.ActorType) {
		validationErrors = append(validationErrors, domain.NewValidationError("actorType", "supported", "must be system, seller, buyer, or approver"))
	}
	if params.Payload == nil {
		validationErrors = append(validationErrors, domain.NewValidationError("payload", "required", "is required"))
	}
	if params.CreatedAt.Time().IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("createdAt", "required", "is required"))
	}
	if signer == nil {
		validationErrors = append(validationErrors, domain.NewValidationError("signer", "required", "is required"))
	}
	if previous == nil {
		if params.Sequence != 1 {
			validationErrors = append(validationErrors, domain.NewValidationError("sequence", "start", "first event sequence must be one"))
		}
	} else {
		if params.TransactionID != previous.TransactionID {
			validationErrors = append(validationErrors, domain.NewValidationError("transactionId", "chain", "must match the previous event"))
		}
		if params.Sequence != previous.Sequence+1 {
			validationErrors = append(validationErrors, domain.NewValidationError("sequence", "contiguous", "must increment the previous sequence by one"))
		}
		if params.CreatedAt.Before(previous.CreatedAt) {
			validationErrors = append(validationErrors, domain.NewValidationError("createdAt", "chronology", "cannot occur before the previous event"))
		}
	}
	if len(validationErrors) > 0 {
		return validationErrors
	}
	return nil
}

func validateStoredEvent(event Event, index int, events []Event) error {
	if event.Sequence != uint64(index+1) || strings.TrimSpace(event.KMSKeyID) == "" || strings.TrimSpace(event.KMSSignature) == "" {
		return ErrChainInvalid
	}
	if index == 0 {
		if event.PreviousEventHash != nil {
			return ErrChainInvalid
		}
		return nil
	}
	previous := events[index-1]
	if event.TransactionID != previous.TransactionID || event.PreviousEventHash == nil || *event.PreviousEventHash != previous.EventHash || event.CreatedAt.Before(previous.CreatedAt) {
		return ErrChainInvalid
	}
	return nil
}

func validEventType(eventType EventType) bool {
	switch eventType {
	case EventIntentCreated, EventApprovalRequested, EventApprovalDecided, EventApprovalResolved,
		EventPaymentChallenged, EventPaymentVerified, EventProxyForwarded, EventDeliverySucceeded,
		EventDeliveryFailed, EventDisputeOpened, EventDisputeClassified, EventDisputeResolved:
		return true
	default:
		return false
	}
}

func validActorType(actorType ActorType) bool {
	return actorType == ActorSystem || actorType == ActorSeller || actorType == ActorBuyer || actorType == ActorApprover
}

func clonePayload(payload map[string]any) map[string]any {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return payload
	}
	var cloned map[string]any
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		return payload
	}
	return cloned
}
