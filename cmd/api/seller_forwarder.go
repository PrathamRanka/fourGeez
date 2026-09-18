package main

import "github.com/fourgeez/agentpay/internal/proxy"

func newSellerForwarder(environment, localSellerEndpoint string) (*proxy.Forwarder, error) {
	if environment == "local" {
		return proxy.NewLocalDevelopmentForwarder(localSellerEndpoint)
	}
	return proxy.NewForwarder(nil), nil
}
