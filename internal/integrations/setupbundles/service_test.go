package setupbundles

import (
	"errors"
	"strings"
	"testing"

	"github.com/fourgeez/agentpay/internal/integrations/stacks"
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
				`"type": "stdio"`,
				`"command": "npx"`,
				`@agentpay/local-mcp-connector@0.1.0`,
				`${AGENTPAY_PROJECT_KEY}`,
			},
		},
		{
			name:           "Codex",
			host:           HostCodex,
			wantConfigPath: ".codex/config.toml",
			wantConfigFragments: []string{
				`[mcp_servers.agentpay]`,
				`command = "npx"`,
				`AGENTPAY_PROJECT_KEY`,
			},
		},
		{
			name:           "generic MCP",
			host:           HostGenericMCP,
			wantConfigPath: "agentpay.mcp.json",
			wantConfigFragments: []string{
				`"transport": "stdio"`,
				`"command": "npx"`,
				`"AGENTPAY_PROJECT_KEY"`,
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
			if len(bundle.Frameworks) != 7 {
				t.Fatalf("framework count = %d, want 7", len(bundle.Frameworks))
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
			if strings.Contains(bundle.Configuration.Template, "Authorization") || strings.Contains(bundle.Configuration.Template, "Bearer") {
				t.Fatal("configuration bypassed the local connector")
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
				"/.well-known/agentpay/sandbox",
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
	if _, err := service.Prompt(HostCodex, Framework("unknown")); !errors.Is(
		err,
		ErrUnsupportedFramework,
	) {
		t.Fatalf("Prompt() error = %v, want ErrUnsupportedFramework", err)
	}
}

// TestServicePublishesStackAwareVersionTwoBundles verifies SEO and discovery instructions.
func TestServicePublishesStackAwareVersionTwoBundles(t *testing.T) {
	t.Parallel()

	service := NewService()
	for _, host := range []Host{HostClaudeCode, HostCodex, HostGenericMCP} {
		bundle, err := service.BundleV2(host)
		if err != nil {
			t.Fatal(err)
		}
		if bundle.SchemaVersion != SchemaVersionV2 || len(bundle.Stacks) != 21 {
			t.Fatalf("bundle = %#v", bundle)
		}
		powerShellSetup := strings.Join(bundle.WindowsPowerShellSetup, "\n")
		for _, fragment := range []string{
			"$env:AGENTPAY_API_BASE_URL",
			"$env:AGENTPAY_PROJECT_KEY",
			"--check",
		} {
			if !strings.Contains(powerShellSetup, fragment) {
				t.Fatalf("PowerShell setup omitted %q: %s", fragment, powerShellSetup)
			}
		}
		if strings.Contains(powerShellSetup, "apc2.") {
			t.Fatal("PowerShell setup embedded a project credential")
		}
		if bundle.Stacks[0].Stack != "nextjs" ||
			!strings.Contains(strings.Join(bundle.Stacks[0].IntegrationNotes, "\n"), "metadata API") {
			t.Fatalf("Next.js stack setup = %#v", bundle.Stacks[0])
		}
		for _, requirement := range []string{
			"canonical",
			"robots",
			"sitemap",
			"JSON-LD",
			"llms.txt",
			"accessibility",
			"performance",
		} {
			if !strings.Contains(strings.Join(bundle.GenerationRequirements, "\n"), requirement) {
				t.Fatalf("bundle omitted %q", requirement)
			}
		}
	}
}

// TestServicePublishesHostConfigurationWithoutMakingOptionalScopesRequired
// verifies first-run host setup remains valid when AGENTPAY_MCP_SCOPES is unset.
func TestServicePublishesHostConfigurationWithoutMakingOptionalScopesRequired(t *testing.T) {
	t.Parallel()

	service := NewService()
	codex, err := service.BundleV2(HostCodex)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(codex.Configuration.Template, "AGENTPAY_MCP_SCOPES") ||
		strings.Contains(codex.Configuration.Template, "default_tools_approval_mode") {
		t.Fatalf("Codex configuration contains unsupported or optional required fields: %s", codex.Configuration.Template)
	}

	generic, err := service.BundleV2(HostGenericMCP)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		`"requiredEnvironmentVariables"`,
		`"optionalEnvironmentVariables"`,
		`"AGENTPAY_MCP_SCOPES"`,
	} {
		if !strings.Contains(generic.Configuration.Template, fragment) {
			t.Fatalf("generic configuration omitted %q: %s", fragment, generic.Configuration.Template)
		}
	}
}

// TestServiceSelectsVersionTwoStackPrompt verifies stack-native and safety guidance.
func TestServiceSelectsVersionTwoStackPrompt(t *testing.T) {
	t.Parallel()

	prompt, err := NewService().PromptV2(HostCodex, "nextjs")
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"Next.js",
		"maintained",
		"detect_repository_stacks",
		"evidenced",
		"app/robots.ts",
		"metadata",
		"canonical",
		"robots",
		"sitemap",
		"JSON-LD",
		"llms.txt",
		"manifest",
		"validate_storefront_artifacts",
		"cannot guarantee ranking",
		"Do not publish",
	} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("prompt omitted %q: %s", fragment, prompt)
		}
	}
	if _, err := NewService().PromptV2(HostCodex, "unknown"); !errors.Is(err, ErrUnsupportedStack) {
		t.Fatalf("PromptV2() error = %v", err)
	}
}

// TestServicePublishesExtendedStackSetup verifies every STK-003 package and recipe.
func TestServicePublishesExtendedStackSetup(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		stack       stacks.Stack
		framework   Framework
		packageName string
		adapter     string
	}{
		{
			stack:       stacks.StackASPNetCore,
			framework:   FrameworkDotNet,
			packageName: "AgentPay.Verify",
			adapter:     "AgentPayVerificationMiddleware",
		},
		{
			stack:       stacks.StackSpringBoot,
			framework:   FrameworkJava,
			packageName: "com.agentpay:agentpay-verify-spring",
			adapter:     "AgentPayVerificationFilter",
		},
		{
			stack:       stacks.StackRails,
			framework:   FrameworkRuby,
			packageName: "agentpay-verify",
			adapter:     "AgentPay::VerificationMiddleware",
		},
		{
			stack:       stacks.StackLaravel,
			framework:   FrameworkPHP,
			packageName: "agentpay/verify",
			adapter:     "AgentPayVerificationMiddleware",
		},
	}

	service := NewService()
	bundle, err := service.BundleV2(HostCodex)
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range testCases {
		t.Run(string(testCase.stack), func(t *testing.T) {
			stackSetup := findStackSetup(t, bundle.Stacks, testCase.stack)
			if stackSetup.Tier != stacks.SupportTierMaintained ||
				stackSetup.Verification == nil ||
				stackSetup.Verification.Framework != testCase.framework ||
				stackSetup.Verification.Package != testCase.packageName ||
				stackSetup.Recipe == nil ||
				stackSetup.Recipe.VerificationAdapter != testCase.adapter {
				t.Fatalf("stack setup = %#v", stackSetup)
			}
			prompt, promptErr := service.PromptV2(HostCodex, string(testCase.stack))
			if promptErr != nil {
				t.Fatal(promptErr)
			}
			for _, fragment := range []string{testCase.packageName, testCase.adapter, "maintained"} {
				if !strings.Contains(prompt, fragment) {
					t.Fatalf("prompt omitted %q: %s", fragment, prompt)
				}
			}
		})
	}
}

// findStackSetup returns one required setup from a versioned bundle.
func findStackSetup(t *testing.T, setups []StackSetup, stack stacks.Stack) StackSetup {
	t.Helper()

	for _, setup := range setups {
		if setup.Stack == stack {
			return setup
		}
	}
	t.Fatalf("stack setup %q was not published", stack)
	return StackSetup{}
}
