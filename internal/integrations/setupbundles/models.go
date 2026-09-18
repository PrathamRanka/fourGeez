package setupbundles

import (
	"github.com/fourgeez/agentpay/internal/integrations/recipes"
	"github.com/fourgeez/agentpay/internal/integrations/stacks"
)

const (
	SchemaVersionV1                = "agentpay.setup.v1"
	SchemaVersionV2                = "agentpay.setup.v2"
	MCPEndpointEnvironmentVariable = "AGENTPAY_MCP_URL"
	CredentialEnvironmentVariable  = "AGENTPAY_INTEGRATION_TOKEN"
	verificationPackageVersion     = "0.1.0"
)

// Host identifies one supported coding-agent setup format.
type Host string

const (
	HostClaudeCode Host = "claude-code"
	HostCodex      Host = "codex"
	HostGenericMCP Host = "generic-mcp"
)

// Framework identifies one maintained seller verification package.
type Framework string

const (
	FrameworkGo     Framework = "go"
	FrameworkNode   Framework = "node"
	FrameworkPython Framework = "python"
)

// Configuration contains the project file template for one MCP host.
type Configuration struct {
	Path     string `json:"path"`
	Template string `json:"template"`
}

// FrameworkSetup contains pinned verification and test instructions.
type FrameworkSetup struct {
	Framework      Framework `json:"framework"`
	Package        string    `json:"package"`
	PackageVersion string    `json:"packageVersion"`
	InstallCommand string    `json:"installCommand"`
	TestCommand    string    `json:"testCommand"`
}

// StackSetup describes the package and stack-native generation conventions.
type StackSetup struct {
	Stack            stacks.Stack       `json:"stack"`
	DisplayName      string             `json:"displayName"`
	Tier             stacks.SupportTier `json:"tier"`
	Verification     *FrameworkSetup    `json:"verification,omitempty"`
	Recipe           *recipes.Recipe    `json:"recipe,omitempty"`
	IntegrationNotes []string           `json:"integrationNotes"`
}

// Bundle is one deterministic, credential-free coding-agent setup contract.
type Bundle struct {
	SchemaVersion                  string           `json:"schemaVersion"`
	Host                           Host             `json:"host"`
	MCPEndpointEnvironmentVariable string           `json:"mcpEndpointEnvironmentVariable"`
	CredentialEnvironmentVariable  string           `json:"credentialEnvironmentVariable"`
	Configuration                  Configuration    `json:"configuration"`
	Frameworks                     []FrameworkSetup `json:"frameworks"`
	Workflow                       []string         `json:"workflow"`
	Prompt                         string           `json:"prompt"`
}

// BundleV2 is the stack-aware SEO, AEO, and agent-discovery setup contract.
type BundleV2 struct {
	SchemaVersion                  string        `json:"schemaVersion"`
	Host                           Host          `json:"host"`
	MCPEndpointEnvironmentVariable string        `json:"mcpEndpointEnvironmentVariable"`
	CredentialEnvironmentVariable  string        `json:"credentialEnvironmentVariable"`
	Configuration                  Configuration `json:"configuration"`
	Stacks                         []StackSetup  `json:"stacks"`
	Workflow                       []string      `json:"workflow"`
	GenerationRequirements         []string      `json:"generationRequirements"`
	Prompt                         string        `json:"prompt"`
}
