package proxy

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/transactions"
)

const defaultMaximumResponseBytes int64 = 1024 * 1024

const (
	// ExecutionCapabilityHeader carries the short-lived ES256 seller capability.
	ExecutionCapabilityHeader = "X-AgentPay-Execution-Capability"
	// SellerSignatureHeader carries the base64 HMAC for a forwarded request.
	SellerSignatureHeader = "X-AgentPay-Signature"
	// SellerTimestampHeader carries the signed RFC 3339 timestamp.
	SellerTimestampHeader = "X-AgentPay-Timestamp"
	// SellerTransactionHeader carries the signed AgentPay transaction ID.
	SellerTransactionHeader = "X-AgentPay-Transaction-Id"
)

var (
	// ErrRouteNotAllowed reports a request outside the configured literal route.
	ErrRouteNotAllowed = errors.New("seller route is not allowed")
	// ErrForbiddenTarget reports an upstream target that could enable SSRF.
	ErrForbiddenTarget = errors.New("seller target is forbidden")
	// ErrRedirectForbidden reports a redirect attempt by the seller.
	ErrRedirectForbidden = errors.New("seller redirects are forbidden")
	// ErrRequestTooLarge reports a paid request above the configured body limit.
	ErrRequestTooLarge = errors.New("seller request body is too large")
	// ErrResponseTooLarge reports a seller response above the capture limit.
	ErrResponseTooLarge = errors.New("seller response is too large")
	// ErrResponseContentType reports active or unexpected seller content.
	ErrResponseContentType = errors.New("seller response content type is not allowed")
	// ErrUpstreamTimeout reports a seller request that exceeded its route limit.
	ErrUpstreamTimeout = errors.New("seller request timed out")
	// ErrUpstreamUnavailable reports a seller transport failure.
	ErrUpstreamUnavailable = errors.New("seller is unavailable")
	// ErrSigningUnavailable reports missing or invalid seller signing material.
	ErrSigningUnavailable = errors.New("seller request signing is unavailable")
	// ErrAlreadyForwarded reports a lost conditional forwarding claim.
	ErrAlreadyForwarded = errors.New("transaction was already forwarded")
)

// ForwardRequest contains one already-authorized seller invocation.
type ForwardRequest struct {
	Seller      catalog.Seller
	Route       catalog.PaidRoute
	Method      catalog.RouteMethod
	Path        string
	Body        []byte
	ContentType string
	Signature   SignatureHeaders
}

// ForwardResponse contains bounded seller response bytes and metadata.
type ForwardResponse struct {
	StatusCode  int
	Body        []byte
	ContentType string
}

// SigningInput contains the request values covered by the seller HMAC.
type SigningInput struct {
	TransactionID   domain.ID
	SellerID        domain.ID
	RouteID         domain.ID
	Method          catalog.RouteMethod
	Path            string
	Body            []byte
	PaymentFinality transactions.PaymentFinality
}

// SignatureHeaders contains safe request authentication metadata.
type SignatureHeaders struct {
	ExecutionCapability string
	Signature           string
	Timestamp           string
	Transaction         string
}

// ExecutionRequest contains a verified transaction and its seller request.
type ExecutionRequest struct {
	Transaction transactions.Transaction
	Seller      catalog.Seller
	Route       catalog.PaidRoute
	Method      catalog.RouteMethod
	Path        string
	Body        []byte
	ContentType string
}

// SecretProvider resolves seller secrets without exposing storage details.
type SecretProvider interface {
	GetSecret(context.Context, string) ([]byte, error)
}

// ForwardingRepository atomically claims one verified transaction.
type ForwardingRepository interface {
	ClaimForwarding(
		context.Context,
		domain.ID,
		uint64,
		domain.Timestamp,
	) (transactions.Transaction, bool, error)
	Update(context.Context, transactions.Transaction, uint64) error
}

// RequestSigner signs one claimed seller request.
type RequestSigner interface {
	Sign(context.Context, string, SigningInput) (SignatureHeaders, error)
}

// SellerForwarder sends one request to an already validated seller route.
type SellerForwarder interface {
	Forward(context.Context, ForwardRequest) (ForwardResponse, error)
}

// LifecycleRecorder persists allowlisted forwarding and delivery evidence.
type LifecycleRecorder interface {
	RecordProxyForwarding(
		context.Context,
		domain.ID,
		evidence.ProxyForwardingFacts,
	) error
	RecordDelivery(
		context.Context,
		domain.ID,
		evidence.DeliveryFacts,
	) error
}

// UsageRecorder records successful fulfillment without controlling buyer flow.
type UsageRecorder interface {
	RecordSuccessfulTransactionUsage(context.Context, domain.ID) error
}

// Resolver resolves a hostname at validation and connection time.
type Resolver interface {
	LookupIP(context.Context, string, string) ([]net.IP, error)
}

// HTTPClient executes a prepared upstream request.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// ContextDialer opens a connection to a validated resolved address.
type ContextDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}
