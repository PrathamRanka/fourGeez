package proxy

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/fourgeez/agentpay/internal/catalog"
)

const defaultMaximumResponseBytes int64 = 1024 * 1024

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
)

// ForwardRequest contains one already-authorized seller invocation.
type ForwardRequest struct {
	Seller      catalog.Seller
	Route       catalog.PaidRoute
	Method      catalog.RouteMethod
	Path        string
	Body        []byte
	ContentType string
}

// ForwardResponse contains bounded seller response bytes and metadata.
type ForwardResponse struct {
	StatusCode  int
	Body        []byte
	ContentType string
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
