package catalog

import (
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

func TestNormalizeClosedJSONSchema(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantError bool
	}{
		{name: "closed object", raw: `{"type":"object","properties":{"topic":{"type":"string"}},"required":["topic"],"additionalProperties":false}`},
		{name: "open object", raw: `{"type":"object","properties":{},"additionalProperties":true}`, wantError: true},
		{name: "nested open object", raw: `{"type":"object","properties":{"filters":{"type":"object","properties":{},"additionalProperties":true}},"additionalProperties":false}`, wantError: true},
		{name: "unknown keyword", raw: `{"type":"object","properties":{},"additionalProperties":false,"unevaluatedProperties":false}`, wantError: true},
		{name: "trailing value", raw: `{"type":"object","properties":{},"additionalProperties":false} {}`, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NormalizeClosedJSONSchema([]byte(test.raw))
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestPublishedContractHashBindsPublishedAndExecutionTerms(t *testing.T) {
	route := PaidRoute{
		RouteID:     mustCatalogID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		SellerID:    mustCatalogID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		DisplayName: "Research Report", ProductSlug: "research-report", Description: "Generate a report",
		Method: RouteMethodPost, PathPattern: "/research", MIMEType: "application/json",
		InputSchema:  JSONSchema(`{"additionalProperties":false,"properties":{"topic":{"type":"string"}},"required":["topic"],"type":"object"}`),
		OutputSchema: JSONSchema(`{"additionalProperties":false,"properties":{"summary":{"type":"string"}},"required":["summary"],"type":"object"}`),
		Amount:       domain.MustParseAmount("100000"), Asset: "USDC", Network: "eip155:84532", PayTo: "0x123",
		UpstreamTimeoutSeconds: 20, UpdatedAt: domain.NewTimestamp(time.Date(2026, time.September, 19, 12, 0, 0, 0, time.UTC)), Version: 3,
	}
	baseline, err := PublishedContractHash(route)
	if err != nil {
		t.Fatal(err)
	}
	if len(baseline) != 64 {
		t.Fatalf("hash = %q", baseline)
	}

	tests := []struct {
		name   string
		change func(*PaidRoute)
	}{
		{name: "display name", change: func(candidate *PaidRoute) { candidate.DisplayName = "Premium Research Report" }},
		{name: "product slug", change: func(candidate *PaidRoute) { candidate.ProductSlug = "premium-research-report" }},
		{name: "description", change: func(candidate *PaidRoute) { candidate.Description = "Generate a premium report" }},
		{name: "input schema", change: func(candidate *PaidRoute) { candidate.InputSchema = DefaultClosedObjectSchema }},
		{name: "output schema", change: func(candidate *PaidRoute) { candidate.OutputSchema = DefaultClosedObjectSchema }},
		{name: "amount", change: func(candidate *PaidRoute) { candidate.Amount = domain.MustParseAmount("200000") }},
		{name: "payout destination", change: func(candidate *PaidRoute) { candidate.PayTo = "0x456" }},
		{name: "timeout", change: func(candidate *PaidRoute) { candidate.UpstreamTimeoutSeconds = 25 }},
		{name: "version", change: func(candidate *PaidRoute) { candidate.Version++ }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := route
			test.change(&candidate)
			changed, hashErr := PublishedContractHash(candidate)
			if hashErr != nil {
				t.Fatal(hashErr)
			}
			if changed == baseline {
				t.Fatalf("hash did not bind %s", test.name)
			}
		})
	}
}
