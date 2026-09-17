package analyzer

const (
	RepositoryManifestVersion = "agentpay.repository.v1"
	MaximumOpenAPIBytes       = 512 * 1024
	MaximumRouteProposals     = 50
)

// Framework identifies one supported seller server ecosystem.
type Framework string

const (
	FrameworkGo     Framework = "go"
	FrameworkNode   Framework = "node"
	FrameworkPython Framework = "python"
)

// RepositoryManifest contains only allowlisted repository metadata.
type RepositoryManifest struct {
	SchemaVersion string    `json:"schemaVersion"`
	ServiceName   string    `json:"serviceName"`
	Framework     Framework `json:"framework"`
	OpenAPIPath   string    `json:"openapiPath"`
}

// Request contains the bounded inputs accepted by repository analysis.
type Request struct {
	Manifest RepositoryManifest `json:"manifest"`
	OpenAPI  string             `json:"openapi"`
}

// RouteProposal contains only facts derived from an OpenAPI operation.
type RouteProposal struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Description string `json:"description"`
	MIMEType    string `json:"mimeType"`
}

// RouteRejection explains why one operation cannot become a V1 paid route.
type RouteRejection struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Result contains deterministic proposals without publishing or pricing them.
type Result struct {
	SchemaVersion string           `json:"schemaVersion"`
	ServiceName   string           `json:"serviceName"`
	Framework     Framework        `json:"framework"`
	Proposals     []RouteProposal  `json:"proposals"`
	Rejections    []RouteRejection `json:"rejections"`
}
