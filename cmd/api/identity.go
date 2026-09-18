package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/identity"
)

const localSellerAccessLifetime = 8 * time.Hour

type sellerIdentityConfig struct {
	Environment  string
	LocalToken   string
	LocalSubject string
	AWSRegion    string
	UserPoolID   string
	ClientID     string
	HTTPClient   *http.Client
}

func newSellerIdentityService(
	config sellerIdentityConfig,
	revocations identity.RevocationRepository,
	clock domain.Clock,
) (*identity.Service, error) {
	if revocations == nil || clock == nil {
		return nil, errors.New("seller identity persistence and clock are required")
	}
	if config.Environment == "local" {
		subject := strings.TrimSpace(config.LocalSubject)
		if subject == "" {
			subject = "local-seller"
		}
		token := config.LocalToken
		if token == "" {
			return nil, errors.New("AGENTPAY_LOCAL_SELLER_TOKEN is required in local mode")
		}
		digest := sha256.Sum256([]byte(token))
		identifier := hex.EncodeToString(digest[:])
		adapter := identity.NewLocalAdapter(clock)
		if err := adapter.Register(token, identity.Claims{
			Subject: subject, TokenID: "local-token-" + identifier,
			SessionID: "local-session-" + identifier, IssuedAt: clock.Now(),
			ExpiresAt: clock.Now().Add(localSellerAccessLifetime),
		}); err != nil {
			return nil, fmt.Errorf("configure local seller identity: %w", err)
		}
		return identity.NewService(adapter, revocations, clock), nil
	}
	region := strings.TrimSpace(config.AWSRegion)
	userPoolID := strings.TrimSpace(config.UserPoolID)
	clientID := strings.TrimSpace(config.ClientID)
	if region == "" || userPoolID == "" || clientID == "" {
		return nil, errors.New("AWS_REGION, AGENTPAY_SELLER_USER_POOL_ID, and AGENTPAY_SELLER_USER_POOL_CLIENT_ID are required")
	}
	verifier, err := identity.NewCognitoVerifier(identity.CognitoConfig{
		Issuer:   "https://cognito-idp." + region + ".amazonaws.com/" + userPoolID,
		ClientID: clientID, HTTPClient: config.HTTPClient, Clock: clock,
	})
	if err != nil {
		return nil, fmt.Errorf("configure Cognito seller identity: %w", err)
	}
	return identity.NewService(verifier, revocations, clock), nil
}
