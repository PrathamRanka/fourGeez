package proxy

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
)

// TestForwarderForwardsOnlyConfiguredRoute verifies literal route allowlisting.
func TestForwarderForwardsOnlyConfiguredRoute(t *testing.T) {
	t.Parallel()

	client := &fakeHTTPClient{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(`{"ok":true}`)),
		},
	}
	forwarder := NewForwarderWithClient(
		&staticResolver{addresses: []net.IP{net.ParseIP("93.184.216.34")}},
		client,
		1024,
	)
	request := validForwardRequest(t)
	response, err := forwarder.Forward(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK ||
		string(response.Body) != `{"ok":true}` {
		t.Fatalf("response = %#v", response)
	}
	if client.request == nil ||
		client.request.URL.String() != "https://seller.example/weather" ||
		client.request.Method != http.MethodGet {
		t.Fatalf("request = %#v", client.request)
	}

	request.Path = "/admin"
	if _, err := forwarder.Forward(
		t.Context(),
		request,
	); !errors.Is(err, ErrRouteNotAllowed) {
		t.Fatalf("Forward() path error = %v", err)
	}
}

// TestForwarderRejectsUnsafeResolvedAddresses verifies SSRF address controls.
func TestForwarderRejectsUnsafeResolvedAddresses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		address string
	}{
		{name: "loopback IPv4", address: "127.0.0.1"},
		{name: "private IPv4", address: "10.0.0.1"},
		{name: "metadata", address: "169.254.169.254"},
		{name: "unspecified IPv4", address: "0.0.0.0"},
		{name: "loopback IPv6", address: "::1"},
		{name: "private IPv6", address: "fc00::1"},
		{name: "link local IPv6", address: "fe80::1"},
		{name: "multicast", address: "224.0.0.1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			client := &fakeHTTPClient{}
			forwarder := NewForwarderWithClient(
				&staticResolver{
					addresses: []net.IP{net.ParseIP(test.address)},
				},
				client,
				1024,
			)
			_, err := forwarder.Forward(
				t.Context(),
				validForwardRequest(t),
			)
			if !errors.Is(err, ErrForbiddenTarget) {
				t.Fatalf("Forward() error = %v", err)
			}
			if client.calls.Load() != 0 {
				t.Fatalf("HTTP calls = %d", client.calls.Load())
			}
		})
	}
}

// TestProtectedDialerRejectsDNSRebinding verifies connect-time resolution.
func TestProtectedDialerRejectsDNSRebinding(t *testing.T) {
	t.Parallel()

	resolver := &sequenceResolver{
		responses: [][]net.IP{
			{net.ParseIP("93.184.216.34")},
			{net.ParseIP("127.0.0.1")},
		},
	}
	forwarder := NewForwarderWithClient(
		resolver,
		&fakeHTTPClient{
			response: &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{}`)),
			},
		},
		1024,
	)
	if _, err := forwarder.authorizeTarget(
		t.Context(),
		validForwardRequest(t),
	); err != nil {
		t.Fatal(err)
	}
	dialer := protectedDialer{
		resolver: resolver,
		dialer:   &recordingDialer{},
	}
	if _, err := dialer.DialContext(
		t.Context(),
		"tcp",
		"seller.example:443",
	); !errors.Is(err, ErrForbiddenTarget) {
		t.Fatalf("DialContext() error = %v", err)
	}
}

// TestForwarderRejectsOversizedAndUnsafeResponses verifies response bounds.
func TestForwarderRejectsOversizedAndUnsafeResponses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		response  *http.Response
		wantError error
	}{
		{
			name: "oversized",
			response: &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader("12345")),
			},
			wantError: ErrResponseTooLarge,
		},
		{
			name: "active HTML",
			response: &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"text/html"},
				},
				Body: io.NopCloser(strings.NewReader("<script></script>")),
			},
			wantError: ErrResponseContentType,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			forwarder := NewForwarderWithClient(
				&staticResolver{
					addresses: []net.IP{net.ParseIP("93.184.216.34")},
				},
				&fakeHTTPClient{response: test.response},
				4,
			)
			_, err := forwarder.Forward(
				t.Context(),
				validForwardRequest(t),
			)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("Forward() error = %v", err)
			}
		})
	}
}

// TestRedirectsAreRejected verifies seller redirects are never followed.
func TestRedirectsAreRejected(t *testing.T) {
	t.Parallel()

	if err := rejectRedirect(nil, nil); !errors.Is(err, ErrRedirectForbidden) {
		t.Fatalf("rejectRedirect() error = %v", err)
	}
}

// validForwardRequest creates an allowlisted seller request.
func validForwardRequest(t *testing.T) ForwardRequest {
	t.Helper()

	sellerID, err := domain.ParseID(
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.SellerIDPrefix,
	)
	if err != nil {
		t.Fatal(err)
	}
	routeID, err := domain.ParseID(
		"rte_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.RouteIDPrefix,
	)
	if err != nil {
		t.Fatal(err)
	}
	return ForwardRequest{
		Seller: catalog.Seller{
			SellerID:        sellerID,
			UpstreamBaseURL: "https://seller.example",
			Status:          catalog.SellerStatusActive,
		},
		Route: catalog.PaidRoute{
			RouteID:                routeID,
			SellerID:               sellerID,
			Method:                 catalog.RouteMethodGet,
			PathPattern:            "/weather",
			MIMEType:               "application/json",
			UpstreamTimeoutSeconds: 20,
			Enabled:                true,
		},
		Method: catalog.RouteMethodGet,
		Path:   "/weather",
	}
}

type staticResolver struct {
	addresses []net.IP
	err       error
}

// LookupIP returns the configured DNS result.
func (resolver *staticResolver) LookupIP(
	context.Context,
	string,
	string,
) ([]net.IP, error) {
	return resolver.addresses, resolver.err
}

type sequenceResolver struct {
	responses [][]net.IP
	calls     atomic.Int32
}

// LookupIP returns sequential DNS answers for rebinding tests.
func (resolver *sequenceResolver) LookupIP(
	context.Context,
	string,
	string,
) ([]net.IP, error) {
	index := int(resolver.calls.Add(1)) - 1
	if index >= len(resolver.responses) {
		index = len(resolver.responses) - 1
	}
	return resolver.responses[index], nil
}

type fakeHTTPClient struct {
	response *http.Response
	err      error
	request  *http.Request
	calls    atomic.Int32
}

// Do records the request and returns the configured response.
func (client *fakeHTTPClient) Do(request *http.Request) (*http.Response, error) {
	client.calls.Add(1)
	client.request = request
	return client.response, client.err
}

type recordingDialer struct {
	calls atomic.Int32
}

// DialContext records calls without opening a network connection.
func (dialer *recordingDialer) DialContext(
	context.Context,
	string,
	string,
) (net.Conn, error) {
	dialer.calls.Add(1)
	return nil, errors.New("unexpected dial")
}
