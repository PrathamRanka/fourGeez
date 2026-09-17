package notifications

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

const (
	webhookDeliveryTimeout      = 10 * time.Second
	maximumWebhookResponseBytes = 64 * 1024
	webhookIDHeader             = "X-AgentPay-Webhook-Id"
	webhookTimestampHeader      = "X-AgentPay-Webhook-Timestamp"
	webhookSignatureHeader      = "X-AgentPay-Webhook-Signature"
)

// DeliveryResolver resolves webhook hostnames at validation and connection time.
type DeliveryResolver interface {
	LookupIP(context.Context, string, string) ([]net.IP, error)
}

// DeliveryHTTPClient executes one prepared webhook request.
type DeliveryHTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// WebhookSendResult contains bounded response metadata only.
type WebhookSendResult struct {
	StatusCode       int
	ResponseBodyHash string
}

// WebhookSender validates and sends signed webhook requests.
type WebhookSender struct {
	resolver DeliveryResolver
	client   DeliveryHTTPClient
}

// NewWebhookSender creates a production sender with connect-time DNS checks.
func NewWebhookSender(resolver DeliveryResolver) *WebhookSender {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	dialer := &webhookProtectedDialer{
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
	return &WebhookSender{
		resolver: resolver,
		client: &http.Client{
			Transport:     transport,
			CheckRedirect: rejectWebhookRedirect,
		},
	}
}

// NewWebhookSenderWithClient creates a sender with an explicit HTTP boundary.
func NewWebhookSenderWithClient(
	resolver DeliveryResolver,
	client DeliveryHTTPClient,
) *WebhookSender {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return &WebhookSender{resolver: resolver, client: client}
}

// Send validates the current DNS result and sends one signed canonical event.
func (sender *WebhookSender) Send(
	ctx context.Context,
	endpointURL string,
	event SignedEvent,
) (WebhookSendResult, error) {
	target, err := url.Parse(endpointURL)
	if err != nil || target.Scheme != "https" || target.Hostname() == "" ||
		target.User != nil || target.Fragment != "" {
		return WebhookSendResult{}, ErrWebhookForbiddenTarget
	}
	addresses, err := sender.resolver.LookupIP(ctx, "ip", target.Hostname())
	if err != nil || !webhookAddressesArePublic(addresses) {
		return WebhookSendResult{}, ErrWebhookForbiddenTarget
	}
	if sender.client == nil {
		return WebhookSendResult{}, ErrWebhookUnavailable
	}
	requestContext, cancel := context.WithTimeout(ctx, webhookDeliveryTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(
		requestContext,
		http.MethodPost,
		target.String(),
		bytes.NewReader(event.Body),
	)
	if err != nil {
		return WebhookSendResult{}, ErrWebhookForbiddenTarget
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(webhookIDHeader, event.Headers.EventID)
	request.Header.Set(webhookTimestampHeader, event.Headers.Timestamp)
	request.Header.Set(webhookSignatureHeader, event.Headers.Signature)

	response, err := sender.client.Do(request)
	if err != nil {
		if errors.Is(requestContext.Err(), context.DeadlineExceeded) ||
			errors.Is(err, context.DeadlineExceeded) {
			return WebhookSendResult{}, ErrWebhookTimeout
		}
		return WebhookSendResult{}, ErrWebhookUnavailable
	}
	if response == nil || response.Body == nil {
		return WebhookSendResult{}, ErrWebhookUnavailable
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maximumWebhookResponseBytes+1))
	if err != nil {
		return WebhookSendResult{}, ErrWebhookUnavailable
	}
	if len(body) > maximumWebhookResponseBytes {
		return WebhookSendResult{}, ErrWebhookResponseTooLarge
	}
	digest := sha256.Sum256(body)
	return WebhookSendResult{
		StatusCode:       response.StatusCode,
		ResponseBodyHash: hex.EncodeToString(digest[:]),
	}, nil
}

// webhookProtectedDialer rejects DNS rebinding before opening a connection.
type webhookProtectedDialer struct {
	resolver DeliveryResolver
	dialer   interface {
		DialContext(context.Context, string, string) (net.Conn, error)
	}
}

// DialContext re-resolves and pins one public address.
func (dialer *webhookProtectedDialer) DialContext(
	ctx context.Context,
	network string,
	address string,
) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, ErrWebhookForbiddenTarget
	}
	addresses, err := dialer.resolver.LookupIP(ctx, "ip", host)
	if err != nil || !webhookAddressesArePublic(addresses) {
		return nil, ErrWebhookForbiddenTarget
	}
	return dialer.dialer.DialContext(
		ctx,
		network,
		net.JoinHostPort(addresses[0].String(), port),
	)
}

// webhookAddressesArePublic rejects empty, unsafe, and mixed DNS answers.
func webhookAddressesArePublic(addresses []net.IP) bool {
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

// rejectWebhookRedirect prevents redirect-based target substitution.
func rejectWebhookRedirect(
	_ *http.Request,
	_ []*http.Request,
) error {
	return ErrWebhookRedirectForbidden
}
