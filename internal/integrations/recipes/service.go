package recipes

import (
	"errors"
	"strings"

	"github.com/fourgeez/agentpay/internal/integrations/stacks"
)

var ErrRecipeUnavailable = errors.New("integration recipe is unavailable")

// Service returns maintained stack integration recipes.
type Service struct{}

// NewService creates the deterministic recipe catalog.
func NewService() *Service {
	return &Service{}
}

// Recipe returns one isolated maintained recipe.
func (service *Service) Recipe(stack stacks.Stack) (Recipe, error) {
	recipe, exists := javaScriptRecipes()[stack]
	if !exists {
		return Recipe{}, ErrRecipeUnavailable
	}
	recipe.MiddlewareOrder = append([]string(nil), recipe.MiddlewareOrder...)
	recipe.StorefrontFiles = append([]string(nil), recipe.StorefrontFiles...)
	recipe.DiscoveryFiles = append([]string(nil), recipe.DiscoveryFiles...)
	return recipe, nil
}

// Validate enforces the safety and completeness contract for one recipe.
func Validate(recipe Recipe) error {
	if recipe.SchemaVersion != RecipeSchemaVersion || recipe.Stack == "" {
		return errors.New("recipe identity is invalid")
	}
	if recipe.Language == "" || strings.TrimSpace(recipe.VerificationPackage) == "" ||
		strings.TrimSpace(recipe.PackageVersion) == "" || strings.TrimSpace(recipe.InstallCommand) == "" ||
		strings.TrimSpace(recipe.VerificationAdapter) == "" || strings.TrimSpace(recipe.RawBodyStrategy) == "" {
		return errors.New("recipe verification setup is incomplete")
	}
	wantOrder := []string{
		"capture_raw_body",
		"verify_agentpay_signature",
		"claim_replay_identifier",
		"fulfillment_handler",
	}
	if !equalStrings(recipe.MiddlewareOrder, wantOrder) {
		return errors.New("recipe middleware order is unsafe")
	}
	if recipe.SandboxRoute != SandboxRoute {
		return errors.New("recipe sandbox route is invalid")
	}
	if len(recipe.StorefrontFiles) == 0 || strings.TrimSpace(recipe.FocusedTestCommand) == "" {
		return errors.New("recipe storefront or test instructions are incomplete")
	}
	for _, requiredFile := range []string{
		"robots.txt",
		"sitemap.xml",
		"llms.txt",
		"manifest.json",
	} {
		if !containsString(recipe.DiscoveryFiles, requiredFile) {
			return errors.New("recipe discovery files are incomplete")
		}
	}
	return nil
}

// javaScriptRecipes returns maintained Node verification integrations.
func javaScriptRecipes() map[stacks.Stack]Recipe {
	return map[stacks.Stack]Recipe{
		stacks.StackNextJS: nodeRecipe(
			stacks.StackNextJS,
			"verifyAgentPayRequest",
			"Read request.arrayBuffer() in a Route Handler before decoding JSON.",
			[]string{"app/layout.tsx", "app/products/[slug]/page.tsx", "app/api/agentpay/sandbox/route.ts"},
		),
		stacks.StackReactVite: nodeRecipe(
			stacks.StackReactVite,
			"verifyAgentPayRequest",
			"Verify raw bytes in the companion Node API before any JSON body parser.",
			[]string{"index.html", "src/routes/ProductPage.tsx", "server/agentpay.ts"},
		),
		stacks.StackRemix: nodeRecipe(
			stacks.StackRemix,
			"verifyAgentPayRequest",
			"Read request.arrayBuffer() in the action before calling request.json().",
			[]string{"app/root.tsx", "app/routes/products.$slug.tsx", "app/routes/.well-known.agentpay.sandbox.ts"},
		),
		stacks.StackNuxt: nodeRecipe(
			stacks.StackNuxt,
			"verifyAgentPayRequest",
			"Read the Nitro event raw body before parsing request content.",
			[]string{"nuxt.config.ts", "pages/products/[slug].vue", "server/routes/.well-known/agentpay/sandbox.post.ts"},
		),
		stacks.StackSvelteKit: nodeRecipe(
			stacks.StackSvelteKit,
			"verifyAgentPayRequest",
			"Read Request.arrayBuffer() before consuming form or JSON helpers.",
			[]string{"src/routes/+layout.svelte", "src/routes/products/[slug]/+page.svelte", "src/routes/.well-known/agentpay/sandbox/+server.ts"},
		),
		stacks.StackAstro: nodeRecipe(
			stacks.StackAstro,
			"verifyAgentPayRequest",
			"Read Astro.request.arrayBuffer() before decoding the request body.",
			[]string{"src/layouts/Layout.astro", "src/pages/products/[slug].astro", "src/pages/.well-known/agentpay/sandbox.ts"},
		),
		stacks.StackExpress: nodeRecipe(
			stacks.StackExpress,
			"createAgentPayMiddleware",
			"Capture request.rawBody with express.raw or the body-parser verify callback before JSON parsing.",
			[]string{"src/agentpay/storefront.ts", "src/agentpay/sandbox.ts"},
		),
		stacks.StackFastify: nodeRecipe(
			stacks.StackFastify,
			"verifyAgentPayRequest",
			"Capture the raw payload in an onRequest or preParsing hook before content parsing.",
			[]string{"src/agentpay/storefront.ts", "src/agentpay/sandbox.ts"},
		),
		stacks.StackNestJS: nodeRecipe(
			stacks.StackNestJS,
			"verifyAgentPayRequest",
			"Enable rawBody and verify in a guard before controllers or pipes perform side effects.",
			[]string{"src/agentpay/storefront.controller.ts", "src/agentpay/agentpay.guard.ts", "src/agentpay/sandbox.controller.ts"},
		),
	}
}

// nodeRecipe constructs one pinned Node integration recipe.
func nodeRecipe(
	stack stacks.Stack,
	adapter string,
	rawBodyStrategy string,
	storefrontFiles []string,
) Recipe {
	return Recipe{
		SchemaVersion:       RecipeSchemaVersion,
		Stack:               stack,
		Language:            LanguageNode,
		VerificationPackage: "@agentpay/verify-node",
		PackageVersion:      "0.1.0",
		InstallCommand:      "npm install --save-exact @agentpay/verify-node@0.1.0",
		VerificationAdapter: adapter,
		RawBodyStrategy:     rawBodyStrategy,
		MiddlewareOrder: []string{
			"capture_raw_body",
			"verify_agentpay_signature",
			"claim_replay_identifier",
			"fulfillment_handler",
		},
		SandboxRoute:    SandboxRoute,
		StorefrontFiles: storefrontFiles,
		DiscoveryFiles: []string{
			"robots.txt",
			"sitemap.xml",
			"llms.txt",
			"manifest.json",
		},
		FocusedTestCommand: "npm test -- agentpay.integration.test.ts",
	}
}

// equalStrings compares ordered recipe steps.
func equalStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// containsString reports whether a list contains an exact value.
func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
