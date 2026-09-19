package stacks

const (
	MaximumEvidenceFiles   = 100
	MaximumEvidenceBytes   = 1024 * 1024
	DetectionSchemaVersion = "agentpay.stack-detection.v1"
)

// Stack identifies one application framework or server integration target.
type Stack string

const (
	StackNextJS     Stack = "nextjs"
	StackReactVite  Stack = "react-vite"
	StackRemix      Stack = "remix"
	StackNuxt       Stack = "nuxt"
	StackSvelteKit  Stack = "sveltekit"
	StackAstro      Stack = "astro"
	StackExpress    Stack = "express"
	StackFastify    Stack = "fastify"
	StackNestJS     Stack = "nestjs"
	StackGoNetHTTP  Stack = "go-net-http"
	StackGin        Stack = "gin"
	StackEcho       Stack = "echo"
	StackFiber      Stack = "fiber"
	StackFastAPI    Stack = "fastapi"
	StackStarlette  Stack = "starlette"
	StackFlask      Stack = "flask"
	StackDjango     Stack = "django"
	StackASPNetCore Stack = "aspnet-core"
	StackSpringBoot Stack = "spring-boot"
	StackRails      Stack = "rails"
	StackLaravel    Stack = "laravel"
)

// SupportTier identifies whether AgentPay may advertise a stack integration.
type SupportTier string

const (
	SupportTierMaintained  SupportTier = "maintained"
	SupportTierPlanned     SupportTier = "planned"
	SupportTierUnsupported SupportTier = "unsupported"
)

// MatrixEntry is one explicit stack support declaration.
type MatrixEntry struct {
	Stack       Stack       `json:"stack"`
	DisplayName string      `json:"displayName"`
	Tier        SupportTier `json:"tier"`
}

// Detection records one evidenced stack and the files that established it.
type Detection struct {
	Stack    Stack       `json:"stack"`
	Tier     SupportTier `json:"tier"`
	Evidence []string    `json:"evidence"`
}

// DetectionRequest contains only caller-supplied committed repository evidence.
type DetectionRequest struct {
	Files map[string]string `json:"files"`
}

// DetectionResult is the versioned deterministic maintained-stack result.
type DetectionResult struct {
	SchemaVersion string      `json:"schemaVersion"`
	Detections    []Detection `json:"detections"`
}
