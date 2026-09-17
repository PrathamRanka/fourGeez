package setupbundles

import (
	"errors"
	"strings"
	"testing"
)

// TestServicePublishesVersionedHostBundles verifies every supported host bundle.
func TestServicePublishesVersionedHostBundles(t *testing.T) {
	t.Parallel()

	service := NewService()
	testCases := []struct {
		name                string
		host                Host
		wantConfigPath      string
		wantConfigFragments []string
	}{
		{
			name:           "Claude Code",
			host:           HostClaudeCode,
			wantConfigPath: ".mcp.json",
			wantConfigFragments: []string{
				`"type": "http"`,
				`${AGENTPAY_MCP_URL}`,
				`Bearer ${AGENTPAY_INTEGRATION_TOKEN}`,
			},
		},
		{
			name:           "Codex",
			host:           HostCodex,
			wantConfigPath: ".codex/config.toml",
			wantConfigFragments: []string{
				`[mcp_servers.agentpay]`,
				`bearer_token_env_var = "AGENTPAY_INTEGRATION_TOKEN"`,
			},
		},
		{
			name:           "generic MCP",
			host:           HostGenericMCP,
			wantConfigPath: "agentpay.mcp.json",
			wantConfigFragments: []string{
				`"transport": "streamable-http"`,
				`"tokenEnvironmentVariable": "AGENTPAY_INTEGRATION_TOKEN"`,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			bundle, err := service.Bundle(testCase.host)
			if err != nil {
				t.Fatal(err)
			}
			if bundle.SchemaVersion != SchemaVersionV1 {
				t.Fatalf(
					"SchemaVersion = %q, want %q",
					bundle.SchemaVersion,
					SchemaVersionV1,
				)
			}
			if bundle.Host != testCase.host {
				t.Fatalf("Host = %q, want %q", bundle.Host, testCase.host)
			}
			if bundle.Configuration.Path != testCase.wantConfigPath {
				t.Fatalf(
					"configuration path = %q, want %q",
					bundle.Configuration.Path,
					testCase.wantConfigPath,
				)
			}
			for _, fragment := range testCase.wantConfigFragments {
				if !strings.Contains(bundle.Configuration.Template, fragment) {
					t.Fatalf(
						"configuration omitted %q: %s",
						fragment,
						bundle.Configuration.Template,
					)
				}
			}
			if bundle.CredentialEnvironmentVariable != CredentialEnvironmentVariable {
				t.Fatalf(
					"credential environment variable = %q",
					bundle.CredentialEnvironmentVariable,
				)
			}
			if len(bundle.Frameworks) != 3 {
				t.Fatalf("framework count = %d, want 3", len(bundle.Frameworks))
			}
			if len(bundle.Workflow) < 8 {
				t.Fatalf("workflow step count = %d, want at least 8", len(bundle.Workflow))
			}
			if !strings.Contains(bundle.Prompt, "explicit seller confirmation") {
				t.Fatalf("prompt omitted publication boundary: %s", bundle.Prompt)
			}
			if strings.Contains(bundle.Configuration.Template, "integration_test_token") {
				t.Fatal("configuration embedded raw credential material")
			}
		})
	}
}

// TestServiceSelectsFrameworkPrompt verifies package and test instructions.
func TestServiceSelectsFrameworkPrompt(t *testing.T) {
	t.Parallel()

	service := NewService()
	testCases := []struct {
		framework   Framework
		wantPackage string
		wantInstall string
		wantTest    string
	}{
		{
			framework:   FrameworkGo,
			wantPackage: "github.com/fourgeez/agentpay/verification/go",
			wantInstall: "@v0.1.0",
			wantTest:    "go test ./...",
		},
		{
			framework:   FrameworkNode,
			wantPackage: "@agentpay/verify-node",
			wantInstall: "--save-exact",
			wantTest:    "npm test",
		},
		{
			framework:   FrameworkPython,
			wantPackage: "agentpay-verify",
			wantInstall: "==0.1.0",
			wantTest:    "python -m unittest discover -v",
		},
	}

	for _, testCase := range testCases {
		t.Run(string(testCase.framework), func(t *testing.T) {
			t.Parallel()

			prompt, err := service.Prompt(HostCodex, testCase.framework)
			if err != nil {
				t.Fatal(err)
			}
			for _, fragment := range []string{
				testCase.wantPackage,
				testCase.wantInstall,
				testCase.wantTest,
				"modified-body",
				"replay",
				"Do not publish",
			} {
				if !strings.Contains(prompt, fragment) {
					t.Fatalf("prompt omitted %q: %s", fragment, prompt)
				}
			}
		})
	}
}

// TestServiceRejectsUnsupportedSelections keeps setup inputs closed.
func TestServiceRejectsUnsupportedSelections(t *testing.T) {
	t.Parallel()

	service := NewService()
	if _, err := service.Bundle(Host("other")); !errors.Is(err, ErrUnsupportedHost) {
		t.Fatalf("Bundle() error = %v, want ErrUnsupportedHost", err)
	}
	if _, err := service.Prompt(HostCodex, Framework("ruby")); !errors.Is(
		err,
		ErrUnsupportedFramework,
	) {
		t.Fatalf("Prompt() error = %v, want ErrUnsupportedFramework", err)
	}
}
