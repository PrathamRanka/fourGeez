package proxy

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
)

const maximumPaidRequestBytes int64 = 1024 * 1024

const (
	sellerSignatureDomain  = "agentpay.seller-request.v1"
	minimumHMACSecretBytes = 32
)

// Forwarder sends bounded requests only to validated public seller targets.
type Forwarder struct {
	resolver         Resolver
	client           HTTPClient
	maxResponseBytes int64
}

// HMACSigner signs seller requests with secrets resolved by reference.
type HMACSigner struct {
	secretProvider SecretProvider
	clock          domain.Clock
}

// ExecutionService claims a transaction before any seller-side effect.
type ExecutionService struct {
	repository ForwardingRepository
	signer     RequestSigner
	forwarder  SellerForwarder
	clock      domain.Clock
}

// NewHMACSigner creates a per-seller request signer.
func NewHMACSigner(
	secretProvider SecretProvider,
	clock domain.Clock,
) *HMACSigner {
	return &HMACSigner{
		secretProvider: secretProvider,
		clock:          clock,
	}
}

// NewExecutionService creates the exactly-once forwarding service.
func NewExecutionService(
	repository ForwardingRepository,
	signer RequestSigner,
	forwarder SellerForwarder,
	clock domain.Clock,
) *ExecutionService {
	return &ExecutionService{
		repository: repository,
		signer:     signer,
		forwarder:  forwarder,
		clock:      clock,
	}
}

// Execute conditionally claims and invokes the seller at most once.
func (service *ExecutionService) Execute(
	ctx context.Context,
	request ExecutionRequest,
) (ForwardResponse, error) {
	if request.Transaction.SellerID() != request.Seller.SellerID ||
		request.Transaction.RouteID() != request.Route.RouteID ||
		request.Route.SellerID != request.Seller.SellerID {
		return ForwardResponse{}, ErrRouteNotAllowed
	}
	claimed, won, err := service.repository.ClaimForwarding(
		ctx,
		request.Transaction.TransactionID(),
		request.Transaction.Version(),
		domain.NewTimestamp(service.clock.Now()),
	)
	if err != nil {
		return ForwardResponse{}, err
	}
	if !won {
		return ForwardResponse{}, ErrAlreadyForwarded
	}

	signature, err := service.signer.Sign(
		ctx,
		request.Seller.SigningSecretRef,
		SigningInput{
			TransactionID: claimed.TransactionID(),
			Method:        request.Method,
			Path:          request.Path,
			Body:          request.Body,
		},
	)
	if err != nil {
		return ForwardResponse{}, err
	}
	return service.forwarder.Forward(
		ctx,
		ForwardRequest{
			Seller:      request.Seller,
			Route:       request.Route,
			Method:      request.Method,
			Path:        request.Path,
			Body:        request.Body,
			ContentType: request.ContentType,
			Signature:   signature,
		},
	)
}

// Sign creates the canonical timestamped seller authentication headers.
func (signer *HMACSigner) Sign(
	ctx context.Context,
	secretReference string,
	input SigningInput,
) (SignatureHeaders, error) {
	if signer.secretProvider == nil || signer.clock == nil ||
		strings.TrimSpace(secretReference) == "" ||
		input.TransactionID.String() == "" ||
		input.Method == "" || input.Path == "" {
		return SignatureHeaders{}, ErrSigningUnavailable
	}
	secret, err := signer.secretProvider.GetSecret(ctx, secretReference)
	if err != nil || len(secret) < minimumHMACSecretBytes {
		return SignatureHeaders{}, ErrSigningUnavailable
	}
	timestamp := signer.clock.Now().UTC().Format(time.RFC3339Nano)
	return SignatureHeaders{
		Signature:   calculateHMAC(secret, input, timestamp),
		Timestamp:   timestamp,
		Transaction: input.TransactionID.String(),
	}, nil
}

// VerifyHMACSignature verifies seller authentication in constant time.
func VerifyHMACSignature(
	secret []byte,
	input SigningInput,
	headers SignatureHeaders,
) bool {
	if len(secret) < minimumHMACSecretBytes ||
		headers.Timestamp == "" ||
		headers.Transaction != input.TransactionID.String() {
		return false
	}
	expected, err := base64.StdEncoding.DecodeString(
		calculateHMAC(secret, input, headers.Timestamp),
	)
	if err != nil {
		return false
	}
	supplied, err := base64.StdEncoding.DecodeString(headers.Signature)
	if err != nil {
		return false
	}
	return hmac.Equal(supplied, expected)
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
	if request.Signature.Signature != "" {
		upstreamRequest.Header.Set(
			SellerSignatureHeader,
			request.Signature.Signature,
		)
		upstreamRequest.Header.Set(
			SellerTimestampHeader,
			request.Signature.Timestamp,
		)
		upstreamRequest.Header.Set(
			SellerTransactionHeader,
			request.Signature.Transaction,
		)
	}

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

// calculateHMAC signs an unambiguous versioned request representation.
func calculateHMAC(
	secret []byte,
	input SigningInput,
	timestamp string,
) string {
	bodyDigest := sha256.Sum256(input.Body)
	canonical := strings.Join(
		[]string{
			sellerSignatureDomain,
			timestamp,
			string(input.Method),
			input.Path,
			hex.EncodeToString(bodyDigest[:]),
			input.TransactionID.String(),
		},
		"\n",
	)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(canonical))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
