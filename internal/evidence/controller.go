package evidence

import (
	"context"
	"fmt"

	"github.com/fourgeez/agentpay/internal/domain"
)

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
			return fmt.Errorf("%w: event %d: %v", ErrChainInvalid, index+1, err)
		}

		digest, digestBytes, err := calculateHash(event)
		if err != nil || digest != event.EventHash {
			return fmt.Errorf("%w: event %d hash mismatch", ErrChainInvalid, index+1)
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
			return fmt.Errorf("%w: event %d signature mismatch", ErrChainInvalid, index+1)
		}
	}
	return nil
}
