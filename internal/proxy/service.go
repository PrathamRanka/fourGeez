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
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/observability"
	"github.com/fourgeez/agentpay/internal/transactions"
)

const maximumPaidRequestBytes int64 = 1024 * 1024

const (
	sellerSignatureDomain         = "agentpay.seller-request.v1"
	minimumHMACSecretBytes        = 32
	sellerDispatchDeadlineReserve = 2 * time.Second
)

// Forwarder sends bounded requests only to validated public seller targets.
type Forwarder struct {
	resolver               Resolver
	client                 HTTPClient
	maxResponseBytes       int64
	localDevelopmentTarget *localDevelopmentTarget
}

type localDevelopmentTarget struct {
	hostname string
	port     string
}

// HMACSigner signs seller requests with secrets resolved by reference.
type HMACSigner struct {
	secretProvider SecretProvider
	clock          domain.Clock
}

// LocalSecretProvider supplies one process-local development secret.
type LocalSecretProvider struct {
	secret []byte
}

// NewLocalSecretProvider copies a local development seller secret.
func NewLocalSecretProvider(secret []byte) *LocalSecretProvider {
	return &LocalSecretProvider{secret: append([]byte(nil), secret...)}
}

// GetSecret returns a copy without exposing the stored value.
func (provider *LocalSecretProvider) GetSecret(
	context.Context,
	string,
) ([]byte, error) {
	if len(provider.secret) < minimumHMACSecretBytes {
		return nil, ErrSigningUnavailable
	}
	return append([]byte(nil), provider.secret...), nil
}

// ExecutionService claims a transaction before any seller-side effect.
type ExecutionService struct {
	repository ForwardingRepository
	signer     RequestSigner
	forwarder  SellerForwarder
	recorder   LifecycleRecorder
	usage      UsageRecorder
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

// SetUsageRecorder attaches optional seller billing metering.
func (service *ExecutionService) SetUsageRecorder(recorder UsageRecorder) {
	service.usage = recorder
}

// NewExecutionService creates the exactly-once forwarding service.
func NewExecutionService(
	repository ForwardingRepository,
	signer RequestSigner,
	forwarder SellerForwarder,
	recorder LifecycleRecorder,
	clock domain.Clock,
) *ExecutionService {
	return &ExecutionService{
		repository: repository,
		signer:     signer,
		forwarder:  forwarder,
		recorder:   recorder,
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
	if recoveryHash := request.Transaction.RecoveryRequestBodyHash(); recoveryHash.String() != "" {
		requestHash, hashErr := intents.HashRequestBody(request.Body, request.ContentType)
		if hashErr != nil || requestHash != recoveryHash {
			return ForwardResponse{}, ErrRouteNotAllowed
		}
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
	if service.recorder == nil {
		err := errors.New(
			"forwarding evidence recorder is required",
		)
		return ForwardResponse{}, service.failBeforeDispatch(ctx, claimed, "forwarding_evidence_unavailable", err)
	}
	if err := service.recorder.RecordProxyForwarding(
		ctx,
		claimed.TransactionID(),
		evidence.ProxyForwardingFacts{
			SellerID: request.Seller.SellerID.String(),
			RouteID:  request.Route.RouteID.String(),
			Method:   string(request.Method),
			Path:     request.Path,
		},
	); err != nil {
		observability.Record(observability.EventEvidenceFailure)
		return ForwardResponse{}, service.failBeforeDispatch(ctx, claimed, "forwarding_evidence_unavailable", err)
	}

	signature, err := service.signer.Sign(
		ctx,
		request.Seller.SigningSecretRef,
		SigningInput{
			TransactionID:   claimed.TransactionID(),
			SellerID:        claimed.SellerID(),
			RouteID:         claimed.RouteID(),
			Method:          request.Method,
			Path:            request.Path,
			Body:            request.Body,
			PaymentFinality: claimed.PaymentFinality(),
		},
	)
	if err != nil {
		observability.Record(observability.EventSellerForwardingFailure)
		return ForwardResponse{}, service.failBeforeDispatch(ctx, claimed, "execution_authorization_unavailable", err)
	}
	response, err := service.forwarder.Forward(
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
	if err != nil {
		observability.Record(observability.EventSellerForwardingFailure)
		expectedVersion := claimed.Version()
		if stateErr := claimed.MarkFailed(
			deliveryFailureCode(err),
			nil,
			nil,
			domain.NewTimestamp(service.clock.Now()),
		); stateErr != nil {
			return ForwardResponse{}, stateErr
		}
		if stateErr := service.repository.Update(
			ctx,
			claimed,
			expectedVersion,
		); stateErr != nil {
			return ForwardResponse{}, stateErr
		}
		recordError := service.recorder.RecordDelivery(
			ctx,
			claimed.TransactionID(),
			evidence.DeliveryFacts{
				Succeeded:   false,
				FailureCode: deliveryFailureCode(err),
			},
		)
		if recordError != nil {
			observability.Record(observability.EventEvidenceFailure)
			return ForwardResponse{}, recordError
		}
		return ForwardResponse{}, err
	}
	responseDigest := sha256.Sum256(response.Body)
	responseHash, err := intents.ParseSHA256Digest(
		hex.EncodeToString(responseDigest[:]),
	)
	if err != nil {
		return ForwardResponse{}, err
	}
	expectedVersion := claimed.Version()
	succeeded := response.StatusCode >= 200 && response.StatusCode <= 299
	if succeeded {
		err = claimed.MarkFulfilled(
			response.StatusCode,
			responseHash,
			transactions.ResponseSummary{
				ContentType:   response.ContentType,
				ContentLength: int64(len(response.Body)),
			},
			domain.NewTimestamp(service.clock.Now()),
		)
	} else {
		status := response.StatusCode
		err = claimed.MarkFailedWithRecovery(
			"upstream_status",
			&status,
			&responseHash,
			response.RetrySafe,
			domain.NewTimestamp(service.clock.Now()),
		)
	}
	if err != nil {
		return ForwardResponse{}, err
	}
	if err := service.repository.Update(
		ctx,
		claimed,
		expectedVersion,
	); err != nil {
		return ForwardResponse{}, err
	}
	if err := service.recorder.RecordDelivery(
		ctx,
		claimed.TransactionID(),
		evidence.DeliveryFacts{
			Succeeded:     succeeded,
			StatusCode:    response.StatusCode,
			ResponseHash:  responseHash,
			ContentType:   response.ContentType,
			ContentLength: int64(len(response.Body)),
		},
	); err != nil {
		observability.Record(observability.EventEvidenceFailure)
		return ForwardResponse{}, err
	}
	if !succeeded {
		observability.Record(observability.EventSellerForwardingFailure)
	}
	if succeeded && service.usage != nil {
		_ = service.usage.RecordSuccessfulTransactionUsage(
			ctx,
			claimed.TransactionID(),
		)
	}
	return response, nil
}

func (service *ExecutionService) failBeforeDispatch(ctx context.Context, transaction transactions.Transaction, failureCode string, cause error) error {
	expectedVersion := transaction.Version()
	if err := transaction.MarkFailedWithRecovery(failureCode, nil, nil, true, domain.NewTimestamp(service.clock.Now())); err != nil {
		return err
	}
	if err := service.repository.Update(ctx, transaction, expectedVersion); err != nil {
		return err
	}
	return cause
}

// deliveryFailureCode maps transport errors to stable evidence values.
func deliveryFailureCode(err error) string {
	if errors.Is(err, ErrUpstreamTimeout) {
		return "upstream_timeout"
	}
	if errors.Is(err, ErrResponseTooLarge) {
		return "response_too_large"
	}
	if errors.Is(err, ErrResponseContentType) {
		return "response_content_type"
	}
	return "upstream_unavailable"
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
	return newForwarder(resolver, nil)
}

// NewLocalDevelopmentForwarder creates a forwarder that additionally permits
// one exact loopback origin. Production wiring must always use NewForwarder.
func NewLocalDevelopmentForwarder(rawEndpoint string) (*Forwarder, error) {
	target, err := parseLocalDevelopmentTarget(rawEndpoint)
	if err != nil {
		return nil, err
	}
	return newForwarder(nil, target), nil
}

func newForwarder(resolver Resolver, localTarget *localDevelopmentTarget) *Forwarder {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	dialer := &protectedDialer{
		resolver:               resolver,
		dialer:                 &net.Dialer{},
		localDevelopmentTarget: localTarget,
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
		maxResponseBytes:       defaultMaximumResponseBytes,
		localDevelopmentTarget: localTarget,
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

	upstreamTimeout := time.Duration(request.Route.UpstreamTimeoutSeconds) * time.Second
	if parentDeadline, ok := ctx.Deadline(); ok && time.Until(parentDeadline) < upstreamTimeout+sellerDispatchDeadlineReserve {
		return ForwardResponse{}, ErrUpstreamUnavailable
	}
	requestContext, cancel := context.WithTimeout(
		ctx,
		upstreamTimeout,
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
	if request.Signature.ExecutionCapability != "" {
		upstreamRequest.Header.Set(
			ExecutionCapabilityHeader,
			request.Signature.ExecutionCapability,
		)
		upstreamRequest.Header.Set(
			SellerTransactionHeader,
			request.Signature.Transaction,
		)
	} else if request.Signature.Signature != "" {
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
		RetrySafe: (upstreamResponse.StatusCode == http.StatusBadRequest || upstreamResponse.StatusCode == http.StatusUnprocessableEntity) &&
			strings.EqualFold(strings.TrimSpace(upstreamResponse.Header.Get(SellerRetrySafeHeader)), SellerRetrySafeCorrectedInput),
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
	if err != nil ||
		baseURL.Host == "" || baseURL.User != nil ||
		baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return nil, ErrForbiddenTarget
	}
	localDevelopmentTarget := forwarder.localDevelopmentTarget != nil &&
		forwarder.localDevelopmentTarget.matchesURL(baseURL)
	if baseURL.Scheme != "https" && !localDevelopmentTarget {
		return nil, ErrForbiddenTarget
	}
	addresses, err := forwarder.resolver.LookupIP(
		ctx,
		"ip",
		baseURL.Hostname(),
	)
	if err != nil || localDevelopmentTarget && !addressesAreLoopback(addresses) ||
		!localDevelopmentTarget && !addressesArePublic(addresses) {
		return nil, ErrForbiddenTarget
	}
	baseURL.Path = request.Route.PathPattern
	baseURL.RawPath = ""
	return baseURL, nil
}

// protectedDialer re-resolves and pins the validated IP for each connection.
type protectedDialer struct {
	resolver               Resolver
	dialer                 ContextDialer
	localDevelopmentTarget *localDevelopmentTarget
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
	localDevelopmentTarget := dialer.localDevelopmentTarget != nil &&
		dialer.localDevelopmentTarget.matchesAddress(host, port)
	if err != nil || localDevelopmentTarget && !addressesAreLoopback(addresses) ||
		!localDevelopmentTarget && !addressesArePublic(addresses) {
		return nil, ErrForbiddenTarget
	}
	return dialer.dialer.DialContext(
		ctx,
		network,
		net.JoinHostPort(addresses[0].String(), port),
	)
}

func parseLocalDevelopmentTarget(rawEndpoint string) (*localDevelopmentTarget, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawEndpoint))
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		!isLoopbackHost(parsed.Hostname()) {
		return nil, ErrForbiddenTarget
	}
	port := parsed.Port()
	if port == "" {
		port = "80"
	}
	return &localDevelopmentTarget{
		hostname: strings.ToLower(parsed.Hostname()),
		port:     port,
	}, nil
}

func (target *localDevelopmentTarget) matchesURL(candidate *url.URL) bool {
	if target == nil || candidate == nil || candidate.Scheme != "http" {
		return false
	}
	port := candidate.Port()
	if port == "" {
		port = "80"
	}
	return strings.EqualFold(candidate.Hostname(), target.hostname) && port == target.port
}

func (target *localDevelopmentTarget) matchesAddress(host, port string) bool {
	return target != nil && strings.EqualFold(host, target.hostname) && port == target.port
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func addressesAreLoopback(addresses []net.IP) bool {
	if len(addresses) == 0 {
		return false
	}
	for _, address := range addresses {
		if address == nil || !address.IsLoopback() {
			return false
		}
	}
	return true
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
