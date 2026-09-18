package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/health"
	"github.com/fourgeez/agentpay/internal/proxy"
	"github.com/fourgeez/agentpay/internal/realtime"
)

const (
	defaultDependencyHealthTimeout = 3 * time.Second
	readinessSellerID              = "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"
	readinessTransactionID         = "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7"
	readinessApprovalSessionID     = "aps_01K5D09YJ0C0M7RJM4FWQ0K9H7"
	readinessSigningSecretRef      = "local-readiness"
	maximumReadinessResponseBytes  = 4 << 10
)

type dependencyHealthConfig struct {
	RepositoryMode      string
	PaymentReadinessURL string
	SellerReadinessURL  string
	Timeout             time.Duration
}

type readinessCatalog interface {
	ListRoutesBySeller(context.Context, domain.ID) ([]catalog.PaidRoute, error)
}

type readinessWebSocketRepository interface {
	ListBySession(context.Context, domain.ID) ([]realtime.Connection, error)
}

type dependencyHealthDependencies struct {
	Catalog             readinessCatalog
	EvidenceSigner      evidence.Signer
	SellerSigner        proxy.RequestSigner
	SellerSigningSecret []byte
	WebSocketRepository readinessWebSocketRepository
	HTTPClient          *http.Client
}

func newDependencyHealthController(
	config dependencyHealthConfig,
	dependencies dependencyHealthDependencies,
) (*health.Controller, error) {
	if strings.TrimSpace(config.RepositoryMode) != "memory" {
		return nil, errors.New("dependency health requires configured memory persistence")
	}
	if dependencies.Catalog == nil || dependencies.EvidenceSigner == nil ||
		dependencies.SellerSigner == nil || dependencies.WebSocketRepository == nil {
		return nil, errors.New("dependency health requires all runtime dependencies")
	}
	paymentURL, err := parseReadinessURL(config.PaymentReadinessURL)
	if err != nil {
		return nil, fmt.Errorf("invalid payment readiness URL: %w", err)
	}
	sellerURL, err := parseReadinessURL(config.SellerReadinessURL)
	if err != nil {
		return nil, fmt.Errorf("invalid seller readiness URL: %w", err)
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultDependencyHealthTimeout
	}
	httpClient := dependencies.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	sellerID, err := domain.ParseID(readinessSellerID, domain.SellerIDPrefix)
	if err != nil {
		return nil, err
	}
	transactionID, err := domain.ParseID(readinessTransactionID, domain.TransactionIDPrefix)
	if err != nil {
		return nil, err
	}
	approvalSessionID, err := domain.ParseID(readinessApprovalSessionID, domain.ApprovalIDPrefix)
	if err != nil {
		return nil, err
	}

	return health.NewController([]health.Dependency{
		{
			Name: "persistence",
			Check: func(ctx context.Context) error {
				_, err := dependencies.Catalog.ListRoutesBySeller(ctx, sellerID)
				return err
			},
		},
		{
			Name: "signing",
			Check: signingReadinessCheck(
				dependencies.EvidenceSigner,
				dependencies.SellerSigner,
				dependencies.SellerSigningSecret,
				transactionID,
			),
		},
		{
			Name:  "payment",
			Check: httpReadinessCheck(httpClient, paymentURL, `{"paymentSignature":"mock:approved"}`),
		},
		{
			Name:  "seller_forwarding",
			Check: httpReadinessCheck(httpClient, sellerURL, `{}`),
		},
		{
			Name: "websocket",
			Check: func(ctx context.Context) error {
				_, err := dependencies.WebSocketRepository.ListBySession(ctx, approvalSessionID)
				return err
			},
		},
	}, config.Timeout)
}

func signingReadinessCheck(
	evidenceSigner evidence.Signer,
	sellerSigner proxy.RequestSigner,
	sellerSigningSecret []byte,
	transactionID domain.ID,
) func(context.Context) error {
	return func(ctx context.Context) error {
		digest := []byte("agentpay.readiness.v1")
		signature, err := evidenceSigner.Sign(ctx, digest)
		if err != nil {
			return err
		}
		valid, err := evidenceSigner.Verify(ctx, signature.KeyID, digest, signature.Value)
		if err != nil || !valid {
			return errors.New("evidence signing verification failed")
		}
		input := proxy.SigningInput{
			TransactionID: transactionID,
			Method:        catalog.RouteMethodPost,
			Path:          health.ReadyPath,
			Body:          []byte("{}"),
		}
		headers, err := sellerSigner.Sign(ctx, readinessSigningSecretRef, input)
		if err != nil {
			return err
		}
		if !proxy.VerifyHMACSignature(sellerSigningSecret, input, headers) {
			return errors.New("seller signing verification failed")
		}
		return nil
	}
}

func httpReadinessCheck(
	client *http.Client,
	target *url.URL,
	body string,
) func(context.Context) error {
	return func(ctx context.Context) error {
		request, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			target.String(),
			strings.NewReader(body),
		)
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maximumReadinessResponseBytes))
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			return errors.New("dependency returned a non-success status")
		}
		return nil
	}
}

func parseReadinessURL(rawURL string) (*url.URL, error) {
	target, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, err
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, errors.New("scheme must be HTTP or HTTPS")
	}
	if target.Host == "" || target.User != nil || target.Fragment != "" {
		return nil, errors.New("URL must be absolute and contain no credentials or fragment")
	}
	return target, nil
}
