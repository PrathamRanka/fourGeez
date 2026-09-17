package setupbundles

const (
	SchemaVersionV1                = "agentpay.setup.v1"
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
