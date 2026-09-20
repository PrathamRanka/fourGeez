package setupbundles

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fourgeez/agentpay/internal/integrations/recipes"
	"github.com/fourgeez/agentpay/internal/integrations/stacks"
)

var (
	ErrUnsupportedHost      = errors.New("unsupported coding-agent host")
	ErrUnsupportedFramework = errors.New("unsupported seller framework")
	ErrUnsupportedStack     = errors.New("unsupported seller stack")
	ErrStackUnavailable     = errors.New("seller stack does not have a setup package")
)

const setupPromptTemplate = `You are the seller's coding agent. Edit this repository to connect it to AgentPay using the %s setup bundle and the maintained %s verification package. AgentPay MCP tools provide bounded analysis, configuration, and verification; they do not write repository files.

1. Read the repository instructions and inspect the existing application and tests before editing.
2. Read agentpay://seller, agentpay://routes, and the selected versioned setup resource.
3. Analyze only the allowlisted repository manifest and OpenAPI document with analyze_repository.
4. Install the pinned verification package with: %s
5. Add AgentPay raw-body signature verification before fulfillment. Add a side-effect-free POST /.well-known/agentpay/sandbox endpoint behind the same middleware. Preserve exact method, literal route path, body bytes, timestamp, and transaction identifier. For the sandbox request, return the closed agentpay.sandbox.v2 response with the supplied routeId and routeVersion plus ready: true.
6. Generate storefront discovery and integration code from confirmed published routes without exposing server credentials to browser code.
7. Add focused tests for valid signatures, modified-body rejection, stale requests, replay rejection, and payment gating.
8. Run the repository's existing checks and this focused command: %s
9. Present the proposed routes, validation output, complete diff, and commands for review.

Do not invent or change prices or payout addresses. Do not publish a route, rotate credentials, or deploy production changes without explicit seller confirmation.`

const setupPromptV2Template = `You are the seller's coding agent. Edit this repository to connect it to AgentPay using the %s setup bundle for %s. AgentPay MCP tools provide bounded analysis, configuration, and verification; they do not write repository files. The current support tier is %s and verification uses %s.

Stack-native conventions:
%s

Verification recipe:
%s

1. Read the repository instructions and existing tests before editing.
2. Call detect_repository_stacks with bounded committed evidence. Continue only if this requested stack appears in the result; never guess or select an unevidenced stack.
3. Read agentpay://seller, agentpay://routes, and agentpay://integration/setup/v2/%s.
4. Install the pinned verification package with: %s
5. Add raw-body AgentPay signature verification before fulfillment and a side-effect-free POST /.well-known/agentpay/sandbox endpoint behind the same middleware. Return the closed agentpay.sandbox.v2 response with the supplied routeId and routeVersion plus ready: true.
6. Generate storefront and product pages using the selected stack's native routing, rendering, metadata, robots, and sitemap conventions.
7. Generate truthful title and description metadata, canonical URLs, Open Graph metadata, semantic product content, visible-fact-backed JSON-LD, robots directives, sitemap output, llms.txt, and an AgentPay manifest that agree on every published route.
8. Add focused signature, stale-request, replay, payment-gating, sandbox, metadata, accessibility, performance, llms.txt, and manifest-consistency tests.
9. Measure the generated artifacts and call validate_storefront_artifacts; fix every failed deterministic check.
10. Run the repository's existing checks and this focused command: %s
11. Present route proposals, generated SEO/AEO assets, validation output, complete diff, and commands for seller review.

SEO/AEO work can improve crawlability and machine discovery but cannot guarantee ranking, traffic, or conversion. Do not create hidden text, keyword stuffing, doorway pages, fabricated reviews, unsupported structured data, or claims absent from visible content.

Do not invent or change prices or payout addresses. Do not publish a route, rotate credentials, or deploy production changes without explicit seller confirmation.`

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

// BundleV2 returns the stack-aware setup contract for one supported host.
func (service *Service) BundleV2(host Host) (BundleV2, error) {
	configuration, err := configurationForHost(host)
	if err != nil {
		return BundleV2{}, err
	}
	return BundleV2{
		SchemaVersion:                  SchemaVersionV2,
		Host:                           host,
		MCPEndpointEnvironmentVariable: MCPEndpointEnvironmentVariable,
		CredentialEnvironmentVariable:  CredentialEnvironmentVariable,
		Configuration:                  configuration,
		WindowsPowerShellSetup:         windowsPowerShellSetup(host),
		Stacks:                         stackSetups(),
		Workflow:                       workflowStepsV2(),
		GenerationRequirements:         generationRequirements(),
		Prompt:                         bundlePromptV2(),
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

// PromptV2 selects one host and detected-stack integration workflow.
func (service *Service) PromptV2(host Host, stackName string) (string, error) {
	if _, err := configurationForHost(host); err != nil {
		return "", err
	}
	stackSetup, err := stackSetupFor(stacks.Stack(stackName))
	if err != nil {
		return "", err
	}
	frameworkSetup := *stackSetup.Verification
	recipeSummary := "Use the maintained language verification package and preserve raw request bytes before decoding."
	focusedTestCommand := frameworkSetup.TestCommand
	if stackSetup.Recipe != nil {
		recipeSummary = fmt.Sprintf(
			"Use %s. %s Middleware order: %s.",
			stackSetup.Recipe.VerificationAdapter,
			stackSetup.Recipe.RawBodyStrategy,
			strings.Join(stackSetup.Recipe.MiddlewareOrder, " -> "),
		)
		focusedTestCommand = stackSetup.Recipe.FocusedTestCommand
	}
	return fmt.Sprintf(
		setupPromptV2Template,
		host,
		stackSetup.DisplayName,
		stackSetup.Tier,
		frameworkSetup.Package,
		"- "+strings.Join(stackSetup.IntegrationNotes, "\n- "),
		recipeSummary,
		host,
		frameworkSetup.InstallCommand,
		focusedTestCommand,
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
      "type": "stdio",
      "command": "npx",
      "args": ["--yes", "@agentpay/local-mcp-connector@0.1.0"],
      "env": {
        "AGENTPAY_API_BASE_URL": "${AGENTPAY_API_BASE_URL}",
        "AGENTPAY_PROJECT_KEY": "${AGENTPAY_PROJECT_KEY}"
      }
    }
  }
}`,
		}, nil
	case HostCodex:
		return Configuration{
			Path: ".codex/config.toml",
			Template: `[mcp_servers.agentpay]
command = "npx"
args = ["--yes", "@agentpay/local-mcp-connector@0.1.0"]
env_vars = ["AGENTPAY_API_BASE_URL", "AGENTPAY_PROJECT_KEY"]
required = true`,
		}, nil
	case HostGenericMCP:
		return Configuration{
			Path: "agentpay.mcp.json",
			Template: `{
  "schemaVersion": "agentpay.mcp-connection.v1",
  "name": "agentpay",
  "transport": "stdio",
  "command": "npx",
  "args": ["--yes", "@agentpay/local-mcp-connector@0.1.0"],
  "requiredEnvironmentVariables": ["AGENTPAY_API_BASE_URL", "AGENTPAY_PROJECT_KEY"],
  "optionalEnvironmentVariables": ["AGENTPAY_MCP_SCOPES", "AGENTPAY_REQUEST_TIMEOUT_MS", "AGENTPAY_MAX_MESSAGE_BYTES"]
}`,
		}, nil
	default:
		return Configuration{}, ErrUnsupportedHost
	}
}

// windowsPowerShellSetup returns secret-safe first-run instructions for one host.
func windowsPowerShellSetup(host Host) []string {
	steps := []string{
		`$env:AGENTPAY_API_BASE_URL = "<API base URL shown in AgentPay>"`,
		`$env:AGENTPAY_PROJECT_KEY = "<project key shown once in AgentPay>"`,
		`npx --yes @agentpay/local-mcp-connector@0.1.0 --check`,
	}
	switch host {
	case HostClaudeCode:
		return append(steps, `Start Claude Code from this PowerShell session after saving .mcp.json: claude`)
	case HostCodex:
		return append(steps, `Start Codex from this PowerShell session after saving .codex/config.toml: codex`)
	case HostGenericMCP:
		return append(steps, `Start the generic MCP host from this PowerShell session after importing agentpay.mcp.json.`)
	default:
		return steps
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
		{
			Framework:      FrameworkDotNet,
			Package:        "AgentPay.Verify",
			PackageVersion: verificationPackageVersion,
			InstallCommand: "dotnet add package AgentPay.Verify --version 0.1.0",
			TestCommand:    "dotnet test",
		},
		{
			Framework:      FrameworkJava,
			Package:        "com.agentpay:agentpay-verify-spring",
			PackageVersion: verificationPackageVersion,
			InstallCommand: "./mvnw dependency:get -Dartifact=com.agentpay:agentpay-verify-spring:0.1.0",
			TestCommand:    "./mvnw test",
		},
		{
			Framework:      FrameworkRuby,
			Package:        "agentpay-verify",
			PackageVersion: verificationPackageVersion,
			InstallCommand: "bundle add agentpay-verify --version 0.1.0 --strict",
			TestCommand:    "bundle exec rails test",
		},
		{
			Framework:      FrameworkPHP,
			Package:        "agentpay/verify",
			PackageVersion: verificationPackageVersion,
			InstallCommand: "composer require agentpay/verify:0.1.0",
			TestCommand:    "php artisan test",
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
		"Add raw-body verification and a no-op POST /.well-known/agentpay/sandbox endpoint behind the same middleware, returning the closed agentpay.sandbox.v2 response with ready: true.",
		"Generate storefront discovery and integration code from confirmed routes.",
		"Add signature, freshness, replay, and payment-gating tests.",
		"Run focused tests and the repository's existing quality checks.",
		"Present route proposals, validation output, diff, and commands for review.",
	}
}

// workflowStepsV2 returns the stack-aware integration sequence.
func workflowStepsV2() []string {
	return []string{
		"Read repository instructions and inspect existing code and tests.",
		"Call detect_repository_stacks with bounded committed evidence and select an evidenced stack that owns the paid route.",
		"Read authenticated seller, route, and version-two setup resources.",
		"Install the maintained language verification package.",
		"Add raw-body verification and a no-op POST /.well-known/agentpay/sandbox endpoint returning the closed agentpay.sandbox.v2 response with ready: true.",
		"Generate stack-native storefront, technical SEO, AEO, and agent-discovery assets.",
		"Add signature, sandbox, SEO, accessibility, performance, and consistency tests.",
		"Call validate_storefront_artifacts and fix every failed deterministic check.",
		"Run focused tests and the repository's existing quality checks.",
		"Present route proposals, validation output, generated assets, diff, and commands for review.",
	}
}

// generationRequirements returns deterministic v2 output categories.
func generationRequirements() []string {
	return []string{
		"Stack-native title, description, canonical, Open Graph, and social metadata.",
		"Public robots directives and sitemap entries for visible product pages.",
		"Semantic visible product content and truthful JSON-LD supported by page facts.",
		"Consistent llms.txt and AgentPay storefront manifest route discovery.",
		"Keyboard accessibility, semantic landmarks, labels, and error descriptions.",
		"performance budgets for page weight, blocking scripts, and primary content rendering.",
	}
}

// stackSetups returns package and generation guidance in matrix order.
func stackSetups() []StackSetup {
	matrix := stacks.NewService().Matrix()
	setups := make([]StackSetup, 0, len(matrix))
	for _, entry := range matrix {
		framework, available := frameworkForStack(entry.Stack)
		var verification *FrameworkSetup
		if available {
			frameworkSetup, err := frameworkSetupFor(framework)
			if err == nil {
				verification = &frameworkSetup
			}
		}
		var recipe *recipes.Recipe
		stackRecipe, recipeErr := recipes.NewService().Recipe(entry.Stack)
		if recipeErr == nil {
			recipe = &stackRecipe
		}
		setups = append(setups, StackSetup{
			Stack:            entry.Stack,
			DisplayName:      entry.DisplayName,
			Tier:             entry.Tier,
			Verification:     verification,
			Recipe:           recipe,
			IntegrationNotes: integrationNotesForStack(entry.Stack),
		})
	}
	return setups
}

// stackSetupFor returns one selectable stack setup.
func stackSetupFor(stack stacks.Stack) (StackSetup, error) {
	for _, stackSetup := range stackSetups() {
		if stackSetup.Stack != stack {
			continue
		}
		if stackSetup.Tier == stacks.SupportTierUnsupported ||
			stackSetup.Verification == nil {
			return StackSetup{}, ErrStackUnavailable
		}
		return stackSetup, nil
	}
	return StackSetup{}, ErrUnsupportedStack
}

// frameworkForStack maps supported stacks to maintained language primitives.
func frameworkForStack(stack stacks.Stack) (Framework, bool) {
	switch stack {
	case stacks.StackGoNetHTTP, stacks.StackGin, stacks.StackEcho, stacks.StackFiber:
		return FrameworkGo, true
	case stacks.StackFastAPI, stacks.StackStarlette, stacks.StackFlask, stacks.StackDjango:
		return FrameworkPython, true
	case stacks.StackASPNetCore:
		return FrameworkDotNet, true
	case stacks.StackSpringBoot:
		return FrameworkJava, true
	case stacks.StackRails:
		return FrameworkRuby, true
	case stacks.StackLaravel:
		return FrameworkPHP, true
	default:
		return FrameworkNode, true
	}
}

// integrationNotesForStack returns rendering and discovery conventions.
func integrationNotesForStack(stack stacks.Stack) []string {
	switch stack {
	case stacks.StackNextJS:
		return []string{
			"Use the Next.js metadata API and canonical alternates.",
			"Generate app/robots.ts and app/sitemap.ts from published routes.",
		}
	case stacks.StackReactVite:
		return []string{
			"Render route-specific document head metadata.",
			"Emit public robots.txt, sitemap.xml, llms.txt, and manifest assets.",
		}
	case stacks.StackRemix:
		return []string{
			"Use route meta and links exports.",
			"Expose robots, sitemap, llms.txt, and manifest through resource routes.",
		}
	case stacks.StackNuxt:
		return []string{
			"Use useSeoMeta and useHead for visible route facts.",
			"Generate server routes for robots, sitemap, llms.txt, and manifest.",
		}
	case stacks.StackSvelteKit:
		return []string{
			"Use route load data and svelte:head metadata.",
			"Generate +server routes for discovery documents.",
		}
	case stacks.StackAstro:
		return []string{
			"Generate canonical head metadata in page layouts.",
			"Use Astro routes or integrations for sitemap and discovery documents.",
		}
	case stacks.StackExpress, stacks.StackFastify, stacks.StackNestJS:
		return []string{
			"Preserve raw request bytes before body parsing.",
			"Serve rendered product metadata and public discovery documents from explicit routes.",
		}
	case stacks.StackGoNetHTTP, stacks.StackGin, stacks.StackEcho, stacks.StackFiber:
		return []string{
			"Verify the raw request body before fulfillment handlers.",
			"Serve semantic templates and deterministic discovery endpoints.",
		}
	case stacks.StackFastAPI, stacks.StackStarlette:
		return []string{
			"Verify and replay the ASGI receive body before route handling.",
			"Serve semantic templates and deterministic discovery endpoints.",
		}
	case stacks.StackFlask, stacks.StackDjango:
		return []string{
			"Verify request bytes before business side effects.",
			"Use framework templates and explicit discovery routes.",
		}
	case stacks.StackASPNetCore:
		return []string{
			"Enable request buffering and verify the original body before model binding.",
			"Use Razor metadata plus explicit robots, sitemap, llms.txt, and manifest endpoints.",
		}
	case stacks.StackSpringBoot:
		return []string{
			"Buffer and verify the servlet request in a highest-precedence filter before controller binding.",
			"Use server-rendered templates plus explicit discovery-resource controllers.",
		}
	case stacks.StackRails:
		return []string{
			"Install verification middleware before parameter parsing and restore rack.input after capture.",
			"Use Rails views, route helpers, and public discovery responses generated from published routes.",
		}
	case stacks.StackLaravel:
		return []string{
			"Verify getContent bytes in middleware before controllers or request validation perform side effects.",
			"Use Blade metadata and explicit routes for robots, sitemap, llms.txt, and the manifest.",
		}
	default:
		return []string{
			"A dedicated verification package and fixture are required before this stack can be advertised.",
		}
	}
}

// bundlePromptV2 returns host-neutral SEO and publication safety guidance.
func bundlePromptV2() string {
	return strings.Join([]string{
		"You are the seller's coding agent: edit the repository using the detected stack and maintained language verification package.",
		"AgentPay MCP tools provide bounded analysis, configuration, and verification; they do not write repository files.",
		"Generate stack-native storefront pages, truthful technical SEO, AEO, llms.txt, and manifest output with focused accessibility and performance tests.",
		"Generated changes cannot guarantee ranking and must not use hidden or fabricated content.",
		"Do not invent or change prices or payout addresses, publish routes, rotate credentials, or deploy production changes without explicit seller confirmation.",
	}, " ")
}

// bundlePrompt returns host-neutral safety and integration instructions.
func bundlePrompt() string {
	return `You are the seller's coding agent: edit the repository using the selected framework setup. AgentPay MCP tools provide bounded analysis, configuration, and verification; they do not write repository files. Inspect before editing, install the maintained verification package, generate storefront integration code, add and run tests, then present a reviewable diff. Do not invent or change prices or payout addresses, publish routes, rotate credentials, or deploy production changes without explicit seller confirmation.`
}
