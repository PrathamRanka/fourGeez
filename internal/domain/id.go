package domain

import (
	cryptorand "crypto/rand"
	"io"
	"strings"
	"sync"

	"github.com/oklog/ulid/v2"
)

// IDPrefix identifies the domain type encoded in an ID.
type IDPrefix string

const (
	SellerIDPrefix             IDPrefix = "sel_"
	CredentialIDPrefix         IDPrefix = "key_"
	PaymentDestinationIDPrefix IDPrefix = "dst_"
	RouteIDPrefix              IDPrefix = "rte_"
	IntentIDPrefix             IDPrefix = "int_"
	ApprovalIDPrefix           IDPrefix = "aps_"
	TransactionIDPrefix        IDPrefix = "txn_"
	EvidenceIDPrefix           IDPrefix = "evt_"
	DisputeIDPrefix            IDPrefix = "dsp_"
)

var supportedIDPrefixes = []IDPrefix{
	SellerIDPrefix,
	CredentialIDPrefix,
	PaymentDestinationIDPrefix,
	RouteIDPrefix,
	IntentIDPrefix,
	ApprovalIDPrefix,
	TransactionIDPrefix,
	EvidenceIDPrefix,
	DisputeIDPrefix,
}

// ID is an opaque, prefixed, sortable domain identifier.
type ID string

// String returns the wire representation.
func (identifier ID) String() string {
	return string(identifier)
}

// Prefix returns the known prefix encoded in the identifier.
func (identifier ID) Prefix() IDPrefix {
	raw := identifier.String()
	for _, prefix := range supportedIDPrefixes {
		if strings.HasPrefix(raw, string(prefix)) {
			return prefix
		}
	}
	return ""
}

// ParseID validates a prefixed ULID.
func ParseID(raw string, expectedPrefix IDPrefix) (ID, error) {
	if raw == "" {
		return "", NewValidationError("id", "required", "is required")
	}
	if !expectedPrefix.supported() || !strings.HasPrefix(raw, string(expectedPrefix)) {
		return "", NewValidationError("id", "prefix", "does not match the expected domain prefix")
	}
	if _, err := ulid.ParseStrict(strings.TrimPrefix(raw, string(expectedPrefix))); err != nil {
		return "", NewValidationError("id", "format", "must contain a canonical ULID")
	}
	return ID(raw), nil
}

// IDGenerator creates domain identifiers.
type IDGenerator interface {
	New(prefix IDPrefix) (ID, error)
}

// ULIDGenerator creates monotonic prefixed ULIDs and serializes access to its
// entropy source because monotonic readers are stateful.
type ULIDGenerator struct {
	clock   Clock
	entropy io.Reader
	mutex   sync.Mutex
}

// NewULIDGenerator creates a generator. Nil dependencies select secure defaults.
func NewULIDGenerator(clock Clock, entropy io.Reader) *ULIDGenerator {
	if clock == nil {
		clock = SystemClock{}
	}
	if entropy == nil {
		entropy = cryptorand.Reader
	}
	return &ULIDGenerator{clock: clock, entropy: ulid.Monotonic(entropy, 0)}
}

// New creates a sortable identifier for prefix.
func (generator *ULIDGenerator) New(prefix IDPrefix) (ID, error) {
	if !prefix.supported() {
		return "", NewValidationError("prefix", "unsupported", "is not a supported domain ID prefix")
	}

	generator.mutex.Lock()
	defer generator.mutex.Unlock()

	identifier, err := ulid.New(ulid.Timestamp(generator.clock.Now()), generator.entropy)
	if err != nil {
		return "", err
	}
	return ID(string(prefix) + identifier.String()), nil
}

func (prefix IDPrefix) supported() bool {
	for _, supportedPrefix := range supportedIDPrefixes {
		if prefix == supportedPrefix {
			return true
		}
	}
	return false
}
