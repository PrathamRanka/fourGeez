package main

import "testing"

func TestNewSellerForwarderRequiresExactLoopbackEndpointOnlyInLocalMode(t *testing.T) {
	t.Parallel()

	if _, err := newSellerForwarder("local", "http://127.0.0.1:8090/research/basic"); err != nil {
		t.Fatalf("local forwarder: %v", err)
	}
	if _, err := newSellerForwarder("local", "http://10.0.0.1:8090/research/basic"); err == nil {
		t.Fatal("local forwarder accepted a private non-loopback endpoint")
	}
	if _, err := newSellerForwarder("demo", "http://10.0.0.1:8090/research/basic"); err != nil {
		t.Fatalf("production forwarder should not consume the local endpoint: %v", err)
	}
}
