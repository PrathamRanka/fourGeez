package setupbundles

import (
	"errors"
	"fmt"
)

var (
	ErrUnsupportedHost      = errors.New("unsupported coding-agent host")
	ErrUnsupportedFramework = errors.New("unsupported seller framework")
)

const setupPromptTemplate = `Connect this repository to AgentPay using the %s setup bundle and the maintained %s verification package.

1. Read the repository instructions and inspect the existing application and tests before editing.
2. Read agentpay://seller, agentpay://routes, and the selected versioned setup resource.
3. Analyze only the allowlisted repository manifest and OpenAPI document with analyze_repository.
4. Install the pinned verification package with: %s
5. Add AgentPay raw-body signature verification before fulfillment. Preserve exact method, literal route path, body bytes, timestamp, and transaction identifier.
6. Generate storefront discovery and integration code from confirmed published routes without exposing server credentials to browser code.
7. Add focused tests for valid signatures, modified-body rejection, stale requests, replay rejection, and payment gating.
8. Run the repository's existing checks and this focused command: %s
9. Present the proposed routes, validation output, complete diff, and commands for review.

Do not invent prices. Do not publish a route, rotate credentials, or deploy production changes without explicit seller confirmation.`

// Service assembles immutable coding-agent setup bundles.
type Service struct{}

// NewService creates the deterministic setup-bundle service.
func NewService() *Service {
	return &Service{}
}

// Bundle returns the versioned setup bundle for one supported host.
func (service *Service) Bundle(host Host) (Bundle, error) {
	configuration, err := configurationForHost(host)
	if err != nil {
		return Bundle{}, err
	}
	return Bundle{
		SchemaVersion:                  SchemaVersionV1,
		Host:                           host,
		MCPEndpointEnvironmentVariable: MCPEndpointEnvironmentVariable,
		CredentialEnvironmentVariable:  CredentialEnvironmentVariable,
		Configuration:                  configuration,
		Frameworks:                     frameworkSetups(),
		Workflow:                       workflowSteps(),
		Prompt:                         bundlePrompt(),
	}, nil
}

// Prompt selects one host and framework workflow for an MCP prompt request.
func (service *Service) Prompt(host Host, framework Framework) (string, error) {
	if _, err := configurationForHost(host); err != nil {
		return "", err
	}
	frameworkSetup, err := frameworkSetupFor(framework)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		setupPromptTemplate,
		host,
		frameworkSetup.Package,
		frameworkSetup.InstallCommand,
		frameworkSetup.TestCommand,
	), nil
}

// configurationForHost returns the official project configuration shape.
func configurationForHost(host Host) (Configuration, error) {
	switch host {
	case HostClaudeCode:
		return Configuration{
			Path: ".mcp.json",
			Template: `{
  "mcpServers": {
    "agentpay": {
      "type": "http",
      "url": "${AGENTPAY_MCP_URL}",
      "headers": {
        "Authorization": "Bearer ${AGENTPAY_INTEGRATION_TOKEN}"
      }
    }
  }
}`,
		}, nil
	case HostCodex:
		return Configuration{
			Path: ".codex/config.toml",
			Template: `[mcp_servers.agentpay]
url = "<AGENTPAY_MCP_URL>"
bearer_token_env_var = "AGENTPAY_INTEGRATION_TOKEN"
required = true
default_tools_approval_mode = "writes"`,
		}, nil
	case HostGenericMCP:
		return Configuration{
			Path: "agentpay.mcp.json",
			Template: `{
  "schemaVersion": "agentpay.mcp-connection.v1",
  "name": "agentpay",
  "transport": "streamable-http",
  "urlEnvironmentVariable": "AGENTPAY_MCP_URL",
  "authorization": {
    "type": "bearer",
    "tokenEnvironmentVariable": "AGENTPAY_INTEGRATION_TOKEN"
  }
}`,
		}, nil
	default:
		return Configuration{}, ErrUnsupportedHost
	}
}

// frameworkSetups returns stable framework order for deterministic JSON.
func frameworkSetups() []FrameworkSetup {
	return []FrameworkSetup{
		{
			Framework:      FrameworkGo,
			Package:        "github.com/fourgeez/agentpay/verification/go",
			PackageVersion: verificationPackageVersion,
			InstallCommand: "go get github.com/fourgeez/agentpay/verification/go@v0.1.0",
			TestCommand:    "go test ./...",
		},
		{
			Framework:      FrameworkNode,
			Package:        "@agentpay/verify-node",
			PackageVersion: verificationPackageVersion,
			InstallCommand: "npm install --save-exact @agentpay/verify-node@0.1.0",
			TestCommand:    "npm test",
		},
		{
			Framework:      FrameworkPython,
			Package:        "agentpay-verify",
			PackageVersion: verificationPackageVersion,
			InstallCommand: "python -m pip install agentpay-verify==0.1.0",
			TestCommand:    "python -m unittest discover -v",
		},
	}
}

// frameworkSetupFor returns one maintained verification package selection.
func frameworkSetupFor(framework Framework) (FrameworkSetup, error) {
	for _, frameworkSetup := range frameworkSetups() {
		if frameworkSetup.Framework == framework {
			return frameworkSetup, nil
		}
	}
	return FrameworkSetup{}, ErrUnsupportedFramework
}

// workflowSteps returns the required seller-repository workflow in order.
func workflowSteps() []string {
	return []string{
		"Read repository instructions and inspect existing code and tests.",
		"Read authenticated AgentPay seller, route, and setup resources.",
		"Analyze only the allowlisted repository manifest and OpenAPI document.",
		"Install the maintained verification package for the detected framework.",
		"Add raw-body verification before fulfillment and keep secrets server-side.",
		"Generate storefront discovery and integration code from confirmed routes.",
		"Add signature, freshness, replay, and payment-gating tests.",
		"Run focused tests and the repository's existing quality checks.",
		"Present route proposals, validation output, diff, and commands for review.",
	}
}

// bundlePrompt returns host-neutral safety and integration instructions.
func bundlePrompt() string {
	return `Prepare this repository for AgentPay using the selected framework setup. Inspect before editing, install the maintained verification package, generate storefront integration code, add and run tests, then present a reviewable diff. Do not invent prices, publish routes, rotate credentials, or deploy production changes without explicit seller confirmation.`
}
