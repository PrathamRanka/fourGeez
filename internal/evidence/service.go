package evidence

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/gowebpki/jcs"
)

const evidenceHashDomain = "agentpay.evidence.v1"

const minimumLocalEvidenceSecretBytes = 32

// LocalHMACSigner provides deterministic local signing without production KMS.
type LocalHMACSigner struct {
	keyID  string
	secret []byte
}

// NewLocalHMACSigner validates and copies local evidence signing configuration.
func NewLocalHMACSigner(
	keyID string,
	secret []byte,
) (*LocalHMACSigner, error) {
	trimmedKeyID := strings.TrimSpace(keyID)
	if trimmedKeyID == "" {
		return nil, domain.NewValidationError(
			"evidenceKeyId",
			"required",
			"is required",
		)
	}
	if len(secret) < minimumLocalEvidenceSecretBytes {
		return nil, domain.NewValidationError(
			"evidenceSigningSecret",
			"length",
			"must contain at least 32 bytes",
		)
	}
	return &LocalHMACSigner{
		keyID:  trimmedKeyID,
		secret: append([]byte(nil), secret...),
	}, nil
}

// Sign creates a local HMAC signature for evidence bytes.
func (signer *LocalHMACSigner) Sign(
	_ context.Context,
	digest []byte,
) (Signature, error) {
	mac := hmac.New(sha256.New, signer.secret)
	_, _ = mac.Write(digest)
	return Signature{
		KeyID: signer.keyID,
		Value: base64.StdEncoding.EncodeToString(mac.Sum(nil)),
	}, nil
}

// Verify checks a local evidence signature using constant-time comparison.
func (signer *LocalHMACSigner) Verify(
	ctx context.Context,
	keyID string,
	digest []byte,
	signature string,
) (bool, error) {
	if keyID != signer.keyID {
		return false, nil
	}
	expected, err := signer.Sign(ctx, digest)
	if err != nil {
		return false, err
	}
	return hmac.Equal(
		[]byte(expected.Value),
		[]byte(signature),
	), nil
}

// Append validates, hashes, and signs the next evidence event.
func Append(
	ctx context.Context,
	params EventParams,
	previous *Event,
	signer Signer,
) (Event, error) {
	if err := validateEventParams(params, previous, signer); err != nil {
		return Event{}, err
	}
	event := newUnsignedEvent(params, previous)
	digest, digestBytes, err := calculateHash(event)
	if err != nil {
		return Event{}, err
	}
	signature, err := signer.Sign(ctx, digestBytes)
	if err != nil {
		return Event{}, err
	}
	if signature.KeyID == "" || signature.Value == "" {
		return Event{}, domain.NewValidationError(
			"signature",
			"required",
			"signer must return a key identifier and signature",
		)
	}
	event.EventHash = digest
	event.KMSKeyID = signature.KeyID
	event.KMSSignature = signature.Value
	return event, nil
}

// VerifyChain verifies event ordering, hashes, links, and signatures.
func VerifyChain(ctx context.Context, events []Event, verifier Signer) error {
	if len(events) == 0 || verifier == nil {
		return ErrChainInvalid
	}
	for index, event := range events {
		if err := validateStoredEvent(event, index, events); err != nil {
			return fmt.Errorf(
				"%w: event %d: %v",
				ErrChainInvalid,
				index+1,
				err,
			)
		}
		digest, digestBytes, err := calculateHash(event)
		if err != nil || digest != event.EventHash {
			return fmt.Errorf(
				"%w: event %d hash mismatch",
				ErrChainInvalid,
				index+1,
			)
		}
		valid, err := verifier.Verify(
			ctx,
			event.KMSKeyID,
			digestBytes,
			event.KMSSignature,
		)
		if err != nil {
			return err
		}
		if !valid {
			return fmt.Errorf(
				"%w: event %d signature mismatch",
				ErrChainInvalid,
				index+1,
			)
		}
	}
	return nil
}

// newUnsignedEvent constructs an event before hashing and signing.
func newUnsignedEvent(params EventParams, previous *Event) Event {
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
	return event
}

// calculateHash returns the canonical event digest and raw signing bytes.
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
		return "", nil, domain.NewValidationError(
			"payload",
			"canonical",
			"must contain canonicalizable JSON values",
		)
	}

	hashInput := make([]byte, 0, len(evidenceHashDomain)+1+len(canonical))
	hashInput = append(hashInput, evidenceHashDomain...)
	hashInput = append(hashInput, 0)
	hashInput = append(hashInput, canonical...)
	sum := sha256.Sum256(hashInput)

	digest, err := intents.ParseSHA256Digest(hex.EncodeToString(sum[:]))
	if err != nil {
		return "", nil, err
	}
	return digest, sum[:], nil
}

// validateEventParams validates a new event and its previous link.
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

// validateStoredEvent validates one event's position in a stored chain.
func validateStoredEvent(event Event, index int, events []Event) error {
	if event.Sequence != uint64(index+1) ||
		strings.TrimSpace(event.KMSKeyID) == "" ||
		strings.TrimSpace(event.KMSSignature) == "" {
		return ErrChainInvalid
	}
	if index == 0 {
		if event.PreviousEventHash != nil {
			return ErrChainInvalid
		}
		return nil
	}

	previous := events[index-1]
	if event.TransactionID != previous.TransactionID ||
		event.PreviousEventHash == nil ||
		*event.PreviousEventHash != previous.EventHash ||
		event.CreatedAt.Before(previous.CreatedAt) {
		return ErrChainInvalid
	}
	return nil
}

// validEventType reports whether an event uses the documented vocabulary.
func validEventType(eventType EventType) bool {
	switch eventType {
	case EventIntentCreated,
		EventApprovalRequested,
		EventApprovalDecided,
		EventApprovalResolved,
		EventPaymentChallenged,
		EventPaymentVerified,
		EventProxyForwarded,
		EventDeliverySucceeded,
		EventDeliveryFailed,
		EventDisputeOpened,
		EventDisputeClassified,
		EventDisputeResolved:
		return true
	default:
		return false
	}
}

// validActorType reports whether an actor type is supported.
func validActorType(actorType ActorType) bool {
	return actorType == ActorSystem ||
		actorType == ActorSeller ||
		actorType == ActorBuyer ||
		actorType == ActorApprover
}

// clonePayload protects stored evidence from caller mutation.
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
