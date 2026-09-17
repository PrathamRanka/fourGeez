package proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/fourgeez/agentpay/internal/catalog"
)

const maximumPaidRequestBytes int64 = 1024 * 1024

// Forwarder sends bounded requests only to validated public seller targets.
type Forwarder struct {
	resolver         Resolver
	client           HTTPClient
	maxResponseBytes int64
}

// NewForwarder creates a production forwarder with connect-time DNS checks.
func NewForwarder(resolver Resolver) *Forwarder {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	dialer := &protectedDialer{
		resolver: resolver,
		dialer:   &net.Dialer{},
	}
	transport := &http.Transport{
		DialContext: dialer.DialContext,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		ForceAttemptHTTP2: true,
	}
	return &Forwarder{
		resolver: resolver,
		client: &http.Client{
			Transport:     transport,
			CheckRedirect: rejectRedirect,
		},
		maxResponseBytes: defaultMaximumResponseBytes,
	}
}

// NewForwarderWithClient creates a forwarder with an explicit HTTP boundary.
func NewForwarderWithClient(
	resolver Resolver,
	client HTTPClient,
	maxResponseBytes int64,
) *Forwarder {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	if maxResponseBytes <= 0 {
		maxResponseBytes = defaultMaximumResponseBytes
	}
	return &Forwarder{
		resolver:         resolver,
		client:           client,
		maxResponseBytes: maxResponseBytes,
	}
}

// Forward validates, sends, and bounds one seller request.
func (forwarder *Forwarder) Forward(
	ctx context.Context,
	request ForwardRequest,
) (ForwardResponse, error) {
	target, err := forwarder.authorizeTarget(ctx, request)
	if err != nil {
		return ForwardResponse{}, err
	}
	if int64(len(request.Body)) > maximumPaidRequestBytes {
		return ForwardResponse{}, ErrRequestTooLarge
	}
	if forwarder.client == nil {
		return ForwardResponse{}, ErrUpstreamUnavailable
	}

	requestContext, cancel := context.WithTimeout(
		ctx,
		time.Duration(request.Route.UpstreamTimeoutSeconds)*time.Second,
	)
	defer cancel()
	upstreamRequest, err := http.NewRequestWithContext(
		requestContext,
		string(request.Method),
		target.String(),
		bytes.NewReader(request.Body),
	)
	if err != nil {
		return ForwardResponse{}, ErrRouteNotAllowed
	}
	if request.ContentType != "" {
		upstreamRequest.Header.Set("Content-Type", request.ContentType)
	}
	upstreamRequest.Header.Set("Accept", request.Route.MIMEType)

	upstreamResponse, err := forwarder.client.Do(upstreamRequest)
	if err != nil {
		if errors.Is(requestContext.Err(), context.DeadlineExceeded) ||
			errors.Is(err, context.DeadlineExceeded) {
			return ForwardResponse{}, ErrUpstreamTimeout
		}
		return ForwardResponse{}, ErrUpstreamUnavailable
	}
	if upstreamResponse == nil || upstreamResponse.Body == nil {
		return ForwardResponse{}, ErrUpstreamUnavailable
	}
	defer upstreamResponse.Body.Close()

	contentType := upstreamResponse.Header.Get("Content-Type")
	if !contentTypeAllowed(contentType, request.Route.MIMEType) {
		return ForwardResponse{}, ErrResponseContentType
	}
	body, err := io.ReadAll(
		io.LimitReader(
			upstreamResponse.Body,
			forwarder.maxResponseBytes+1,
		),
	)
	if err != nil {
		return ForwardResponse{}, ErrUpstreamUnavailable
	}
	if int64(len(body)) > forwarder.maxResponseBytes {
		return ForwardResponse{}, ErrResponseTooLarge
	}

	return ForwardResponse{
		StatusCode:  upstreamResponse.StatusCode,
		Body:        body,
		ContentType: contentType,
	}, nil
}

// authorizeTarget validates route allowlisting and current DNS answers.
func (forwarder *Forwarder) authorizeTarget(
	ctx context.Context,
	request ForwardRequest,
) (*url.URL, error) {
	if request.Seller.Status != catalog.SellerStatusActive ||
		!request.Route.Enabled ||
		request.Seller.SellerID != request.Route.SellerID ||
		request.Method != request.Route.Method ||
		request.Path != request.Route.PathPattern {
		return nil, ErrRouteNotAllowed
	}
	baseURL, err := url.Parse(request.Seller.UpstreamBaseURL)
	if err != nil || baseURL.Scheme != "https" ||
		baseURL.Host == "" || baseURL.User != nil ||
		baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return nil, ErrForbiddenTarget
	}
	addresses, err := forwarder.resolver.LookupIP(
		ctx,
		"ip",
		baseURL.Hostname(),
	)
	if err != nil || !addressesArePublic(addresses) {
		return nil, ErrForbiddenTarget
	}
	baseURL.Path = request.Route.PathPattern
	baseURL.RawPath = ""
	return baseURL, nil
}

// protectedDialer re-resolves and pins the validated IP for each connection.
type protectedDialer struct {
	resolver Resolver
	dialer   ContextDialer
}

// DialContext blocks DNS rebinding before opening the socket.
func (dialer *protectedDialer) DialContext(
	ctx context.Context,
	network string,
	address string,
) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, ErrForbiddenTarget
	}
	addresses, err := dialer.resolver.LookupIP(ctx, "ip", host)
	if err != nil || !addressesArePublic(addresses) {
		return nil, ErrForbiddenTarget
	}
	return dialer.dialer.DialContext(
		ctx,
		network,
		net.JoinHostPort(addresses[0].String(), port),
	)
}

// addressesArePublic rejects mixed or empty DNS responses fail-closed.
func addressesArePublic(addresses []net.IP) bool {
	if len(addresses) == 0 {
		return false
	}
	for _, address := range addresses {
		if address == nil || !address.IsGlobalUnicast() ||
			address.IsPrivate() || address.IsLoopback() ||
			address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() ||
			address.IsMulticast() || address.IsUnspecified() {
			return false
		}
	}
	return true
}

// contentTypeAllowed accepts only the route's declared response media type.
func contentTypeAllowed(actual string, expected string) bool {
	actualType, _, actualErr := mime.ParseMediaType(actual)
	expectedType, _, expectedErr := mime.ParseMediaType(expected)
	return actualErr == nil && expectedErr == nil && actualType == expectedType
}

// rejectRedirect prevents redirect-based target substitution.
func rejectRedirect(
	_ *http.Request,
	_ []*http.Request,
) error {
	return ErrRedirectForbidden
}
