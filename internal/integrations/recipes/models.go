package recipes

import "github.com/fourgeez/agentpay/internal/integrations/stacks"

const RecipeSchemaVersion = "agentpay.recipe.v1"

const SandboxRoute = "/.well-known/agentpay/sandbox"

// Language identifies the maintained verification package ecosystem.
type Language string

const (
	LanguageNode   Language = "node"
	LanguageGo     Language = "go"
	LanguagePython Language = "python"
	LanguageDotNet Language = "dotnet"
	LanguageJava   Language = "java"
	LanguageRuby   Language = "ruby"
	LanguagePHP    Language = "php"
)

// Recipe contains one complete stack integration plan.
type Recipe struct {
	SchemaVersion       string       `json:"schemaVersion"`
	Stack               stacks.Stack `json:"stack"`
	Language            Language     `json:"language"`
	VerificationPackage string       `json:"verificationPackage"`
	PackageVersion      string       `json:"packageVersion"`
	InstallCommand      string       `json:"installCommand"`
	VerificationAdapter string       `json:"verificationAdapter"`
	RawBodyStrategy     string       `json:"rawBodyStrategy"`
	MiddlewareOrder     []string     `json:"middlewareOrder"`
	SandboxRoute        string       `json:"sandboxRoute"`
	StorefrontFiles     []string     `json:"storefrontFiles"`
	DiscoveryFiles      []string     `json:"discoveryFiles"`
	FocusedTestCommand  string       `json:"focusedTestCommand"`
}
