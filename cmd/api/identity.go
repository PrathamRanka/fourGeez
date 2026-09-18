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
	Environment        string
	LocalSigningSecret string
	LocalToken         string
	LocalSubject       string
	AWSRegion          string
	UserPoolID         string
	ClientID           string
	HTTPClient         *http.Client
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
		var adapter *identity.LocalAdapter
		var err error
		if config.LocalSigningSecret != "" {
			adapter, err = identity.NewSignedLocalAdapter([]byte(config.LocalSigningSecret), clock)
			if err != nil {
				return nil, fmt.Errorf("configure AGENTPAY_LOCAL_IDENTITY_SIGNING_SECRET: %w", err)
			}
		} else {
			adapter = identity.NewLocalAdapter(clock)
		}
		if config.LocalToken != "" {
			subject := strings.TrimSpace(config.LocalSubject)
			if subject == "" {
				subject = "local-seller"
			}
			digest := sha256.Sum256([]byte(config.LocalToken))
			identifier := hex.EncodeToString(digest[:])
			now := clock.Now()
			if err := adapter.Register(config.LocalToken, identity.Claims{
				Subject: subject, TokenID: "local-token-" + identifier,
				SessionID: "local-session-" + identifier, IssuedAt: now,
				ExpiresAt: now.Add(localSellerAccessLifetime),
			}); err != nil {
				return nil, fmt.Errorf("configure local seller identity: %w", err)
			}
		}
		if config.LocalSigningSecret == "" && config.LocalToken == "" {
			return nil, errors.New("AGENTPAY_LOCAL_IDENTITY_SIGNING_SECRET or AGENTPAY_LOCAL_SELLER_TOKEN is required in local mode")
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
