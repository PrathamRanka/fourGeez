package discovery

const (
	MaximumArtifactPages       = 100
	MaximumArtifactBytes       = 1024 * 1024
	MaximumHTMLBytes           = 200_000
	MaximumJavaScriptBytes     = 250_000
	MaximumCSSBytes            = 100_000
	MaximumBlockingScripts     = 2
	MaximumLargestContentfulMS = 2_500
)

// CheckName identifies one deterministic storefront quality check.
type CheckName string

const (
	CheckMetadata            CheckName = "metadata"
	CheckCanonical           CheckName = "canonical_urls"
	CheckRobots              CheckName = "robots_directives"
	CheckSitemap             CheckName = "sitemap_output"
	CheckStructuredData      CheckName = "structured_data"
	CheckSemanticContent     CheckName = "semantic_content"
	CheckLLMSText            CheckName = "llms_text"
	CheckManifestConsistency CheckName = "manifest_consistency"
	CheckAccessibility       CheckName = "accessibility"
	CheckPerformance         CheckName = "performance_budgets"
)

// PageArtifact contains generated page facts and measured quality signals.
type PageArtifact struct {
	URL                      string `json:"url"`
	RoutePath                string `json:"routePath"`
	Title                    string `json:"title"`
	Description              string `json:"description"`
	CanonicalURL             string `json:"canonicalUrl"`
	Robots                   string `json:"robots"`
	HeadingOne               string `json:"headingOne"`
	VisibleText              string `json:"visibleText"`
	Language                 string `json:"language"`
	MainLandmarks            int    `json:"mainLandmarks"`
	MissingImageAltCount     int    `json:"missingImageAltCount"`
	UnlabelledControls       int    `json:"unlabelledControls"`
	StructuredDataJSON       string `json:"structuredDataJson"`
	HTMLBytes                int    `json:"htmlBytes"`
	JavaScriptBytes          int    `json:"javaScriptBytes"`
	CSSBytes                 int    `json:"cssBytes"`
	BlockingScriptCount      int    `json:"blockingScriptCount"`
	LargestContentfulPaintMS int    `json:"largestContentfulPaintMs"`
}

// Request contains one bounded generated storefront artifact set.
type Request struct {
	BaseURL      string         `json:"baseUrl"`
	Pages        []PageArtifact `json:"pages"`
	RobotsTXT    string         `json:"robotsTxt"`
	SitemapXML   string         `json:"sitemapXml"`
	LLMSText     string         `json:"llmsText"`
	ManifestJSON string         `json:"manifestJson"`
}

// CheckResult reports deterministic pass/fail details without a ranking score.
type CheckResult struct {
	Name   CheckName `json:"name"`
	Passed bool      `json:"passed"`
	Issues []string  `json:"issues"`
}

// Result contains the complete ordered validation outcome.
type Result struct {
	Valid  bool          `json:"valid"`
	Checks []CheckResult `json:"checks"`
}
