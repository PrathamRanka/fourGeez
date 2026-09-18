package discovery

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/url"
	"strings"
)

const minimumVisibleContentCharacters = 100

// Service validates generated storefront and discovery artifacts.
type Service struct{}

// NewService creates the deterministic artifact validator.
func NewService() *Service {
	return &Service{}
}

// Validate executes every quality check in a stable order.
func (service *Service) Validate(request Request) (Result, error) {
	baseURL, err := validateRequestBoundary(request)
	if err != nil {
		return Result{}, err
	}
	checks := []CheckResult{
		validateMetadata(request.Pages),
		validateCanonicalURLs(baseURL, request.Pages),
		validateRobots(baseURL, request),
		validateSitemap(request.Pages, request.SitemapXML),
		validateStructuredData(request.Pages),
		validateSemanticContent(request),
		validateLLMSText(request.Pages, request.LLMSText),
		validateManifestConsistency(request.Pages, request.ManifestJSON),
		validateAccessibility(request.Pages),
		validatePerformance(request.Pages),
	}
	valid := true
	for _, check := range checks {
		if !check.Passed {
			valid = false
		}
	}
	return Result{Valid: valid, Checks: checks}, nil
}

// validateRequestBoundary rejects malformed or unbounded artifact sets.
func validateRequestBoundary(request Request) (*url.URL, error) {
	if len(request.Pages) == 0 || len(request.Pages) > MaximumArtifactPages {
		return nil, errors.New("pages must contain 1-100 generated pages")
	}
	baseURL, err := url.ParseRequestURI(strings.TrimSpace(request.BaseURL))
	if err != nil || baseURL.Scheme != "https" || baseURL.Host == "" {
		return nil, errors.New("baseUrl must be an absolute HTTPS URL")
	}
	totalBytes := len(request.BaseURL) + len(request.RobotsTXT) +
		len(request.SitemapXML) + len(request.LLMSText) + len(request.ManifestJSON)
	for _, page := range request.Pages {
		totalBytes += len(page.URL) + len(page.RoutePath) + len(page.Title) +
			len(page.Description) + len(page.CanonicalURL) + len(page.Robots) +
			len(page.HeadingOne) + len(page.VisibleText) + len(page.Language) +
			len(page.StructuredDataJSON)
	}
	if totalBytes > MaximumArtifactBytes {
		return nil, errors.New("storefront artifacts exceed the 1 MiB limit")
	}
	return baseURL, nil
}

// validateMetadata checks bounded unique titles and descriptions.
func validateMetadata(pages []PageArtifact) CheckResult {
	issues := make([]string, 0)
	titles := make(map[string]struct{})
	descriptions := make(map[string]struct{})
	for _, page := range pages {
		title := strings.TrimSpace(page.Title)
		description := strings.TrimSpace(page.Description)
		if len(title) < 10 || len(title) > 60 {
			issues = append(issues, page.URL+": title must contain 10-60 characters")
		}
		if len(description) < 50 || len(description) > 160 {
			issues = append(issues, page.URL+": description must contain 50-160 characters")
		}
		if _, exists := titles[title]; exists {
			issues = append(issues, page.URL+": title must be unique")
		}
		if _, exists := descriptions[description]; exists {
			issues = append(issues, page.URL+": description must be unique")
		}
		titles[title] = struct{}{}
		descriptions[description] = struct{}{}
	}
	return checkResult(CheckMetadata, issues)
}

// validateCanonicalURLs checks absolute same-origin self-canonical URLs.
func validateCanonicalURLs(baseURL *url.URL, pages []PageArtifact) CheckResult {
	issues := make([]string, 0)
	for _, page := range pages {
		pageURL, pageErr := url.ParseRequestURI(strings.TrimSpace(page.URL))
		canonicalURL, canonicalErr := url.ParseRequestURI(strings.TrimSpace(page.CanonicalURL))
		if pageErr != nil || canonicalErr != nil ||
			pageURL.Scheme != "https" || canonicalURL.Scheme != "https" ||
			pageURL.Host != baseURL.Host || canonicalURL.Host != baseURL.Host ||
			canonicalURL.String() != pageURL.String() {
			issues = append(issues, page.URL+": canonical URL must be an exact same-origin page URL")
		}
	}
	return checkResult(CheckCanonical, issues)
}

// validateRobots checks page directives and public robots discovery.
func validateRobots(baseURL *url.URL, request Request) CheckResult {
	issues := make([]string, 0)
	for _, page := range request.Pages {
		directives := strings.ToLower(strings.ReplaceAll(page.Robots, " ", ""))
		if !strings.Contains(directives, "index") ||
			!strings.Contains(directives, "follow") ||
			strings.Contains(directives, "noindex") ||
			strings.Contains(directives, "nofollow") {
			issues = append(issues, page.URL+": robots metadata must permit indexing and following")
		}
	}
	lowerRobots := strings.ToLower(request.RobotsTXT)
	if strings.Contains(lowerRobots, "disallow: /") {
		issues = append(issues, "robots.txt must not disallow the complete storefront")
	}
	wantSitemap := "sitemap: " + strings.TrimSuffix(baseURL.String(), "/") + "/sitemap.xml"
	if !strings.Contains(lowerRobots, strings.ToLower(wantSitemap)) {
		issues = append(issues, "robots.txt must identify the canonical sitemap")
	}
	return checkResult(CheckRobots, issues)
}

// validateSitemap checks well-formed XML and canonical page coverage.
func validateSitemap(pages []PageArtifact, sitemapXML string) CheckResult {
	var sitemap struct {
		URLs []struct {
			Location string `xml:"loc"`
		} `xml:"url"`
	}
	issues := make([]string, 0)
	if err := xml.Unmarshal([]byte(sitemapXML), &sitemap); err != nil {
		return checkResult(CheckSitemap, []string{"sitemap.xml must contain valid XML"})
	}
	locations := make(map[string]struct{}, len(sitemap.URLs))
	for _, sitemapURL := range sitemap.URLs {
		locations[strings.TrimSpace(sitemapURL.Location)] = struct{}{}
	}
	for _, page := range pages {
		if _, exists := locations[page.CanonicalURL]; !exists {
			issues = append(issues, page.URL+": canonical URL is missing from sitemap.xml")
		}
	}
	return checkResult(CheckSitemap, issues)
}

// validateStructuredData checks truthful visible-fact-backed JSON-LD.
func validateStructuredData(pages []PageArtifact) CheckResult {
	issues := make([]string, 0)
	for _, page := range pages {
		var document map[string]any
		if err := json.Unmarshal([]byte(page.StructuredDataJSON), &document); err != nil {
			issues = append(issues, page.URL+": structured data must contain valid JSON")
			continue
		}
		contextValue, contextValid := document["@context"].(string)
		typeValue, typeValid := document["@type"].(string)
		nameValue, nameValid := document["name"].(string)
		descriptionValue, descriptionValid := document["description"].(string)
		urlValue, urlValid := document["url"].(string)
		if !contextValid || contextValue != "https://schema.org" ||
			!typeValid || (typeValue != "Product" && typeValue != "Service") ||
			!nameValid || nameValue != page.Title ||
			!descriptionValid || descriptionValue != page.Description ||
			!urlValid || urlValue != page.CanonicalURL {
			issues = append(issues, page.URL+": structured data must match visible page facts")
		}
		if _, exists := document["review"]; exists {
			issues = append(issues, page.URL+": generated reviews are not allowed")
		}
		if _, exists := document["aggregateRating"]; exists {
			issues = append(issues, page.URL+": generated ratings are not allowed")
		}
	}
	return checkResult(CheckStructuredData, issues)
}

// validateSemanticContent checks visible page structure and ranking claims.
func validateSemanticContent(request Request) CheckResult {
	issues := make([]string, 0)
	for _, page := range request.Pages {
		if strings.TrimSpace(page.HeadingOne) != strings.TrimSpace(page.Title) {
			issues = append(issues, page.URL+": the single visible heading must match the page title")
		}
		if len(strings.TrimSpace(page.VisibleText)) < minimumVisibleContentCharacters {
			issues = append(issues, page.URL+": visible product content is too short")
		}
		if containsRankingPromise(page.Title + " " + page.Description + " " + page.VisibleText) {
			issues = append(issues, page.URL+": guaranteed ranking claims are prohibited")
		}
	}
	if containsRankingPromise(request.LLMSText) {
		issues = append(issues, "llms.txt contains a prohibited ranking claim")
	}
	return checkResult(CheckSemanticContent, issues)
}

// validateLLMSText checks human-readable agent discovery coverage.
func validateLLMSText(pages []PageArtifact, llmsText string) CheckResult {
	issues := make([]string, 0)
	for _, page := range pages {
		if !strings.Contains(llmsText, page.CanonicalURL) ||
			!strings.Contains(llmsText, page.Title) ||
			!strings.Contains(llmsText, page.Description) {
			issues = append(issues, page.URL+": llms.txt must include canonical title and description")
		}
	}
	return checkResult(CheckLLMSText, issues)
}

// validateManifestConsistency checks enabled route and page agreement.
func validateManifestConsistency(pages []PageArtifact, manifestJSON string) CheckResult {
	var manifest struct {
		Routes []struct {
			PathPattern string `json:"pathPattern"`
			Description string `json:"description"`
			Enabled     bool   `json:"enabled"`
		} `json:"routes"`
	}
	if err := json.Unmarshal([]byte(manifestJSON), &manifest); err != nil {
		return checkResult(CheckManifestConsistency, []string{"manifest must contain valid JSON"})
	}
	issues := make([]string, 0)
	routes := make(map[string]string)
	for _, route := range manifest.Routes {
		if route.Enabled {
			routes[route.PathPattern] = route.Description
		}
	}
	for _, page := range pages {
		description, exists := routes[page.RoutePath]
		if !exists || description != page.Description {
			issues = append(issues, page.URL+": manifest route and visible description must agree")
		}
	}
	if len(routes) != len(pages) {
		issues = append(issues, "manifest enabled-route count must match generated product pages")
	}
	return checkResult(CheckManifestConsistency, issues)
}

// validateAccessibility checks deterministic generated-page accessibility facts.
func validateAccessibility(pages []PageArtifact) CheckResult {
	issues := make([]string, 0)
	for _, page := range pages {
		if strings.TrimSpace(page.Language) == "" {
			issues = append(issues, page.URL+": document language is required")
		}
		if page.MainLandmarks != 1 {
			issues = append(issues, page.URL+": exactly one main landmark is required")
		}
		if page.MissingImageAltCount != 0 {
			issues = append(issues, page.URL+": every informative image requires alt text")
		}
		if page.UnlabelledControls != 0 {
			issues = append(issues, page.URL+": every interactive control requires a label")
		}
	}
	return checkResult(CheckAccessibility, issues)
}

// validatePerformance checks explicit generated-page budgets.
func validatePerformance(pages []PageArtifact) CheckResult {
	issues := make([]string, 0)
	for _, page := range pages {
		if page.HTMLBytes > MaximumHTMLBytes {
			issues = append(issues, page.URL+": HTML exceeds the 200 KB budget")
		}
		if page.JavaScriptBytes > MaximumJavaScriptBytes {
			issues = append(issues, page.URL+": JavaScript exceeds the 250 KB budget")
		}
		if page.CSSBytes > MaximumCSSBytes {
			issues = append(issues, page.URL+": CSS exceeds the 100 KB budget")
		}
		if page.BlockingScriptCount > MaximumBlockingScripts {
			issues = append(issues, page.URL+": blocking script count exceeds two")
		}
		if page.LargestContentfulPaintMS > MaximumLargestContentfulMS {
			issues = append(issues, page.URL+": largest contentful paint exceeds 2500 ms")
		}
	}
	return checkResult(CheckPerformance, issues)
}

// containsRankingPromise rejects deterministic prohibited marketing claims.
func containsRankingPromise(content string) bool {
	lowerContent := strings.ToLower(content)
	for _, phrase := range []string{
		"guaranteed ranking",
		"guaranteed to rank",
		"rank #1",
		"rank number one",
		"top of google",
	} {
		if strings.Contains(lowerContent, phrase) {
			return true
		}
	}
	return false
}

// checkResult constructs one stable named validation result.
func checkResult(name CheckName, issues []string) CheckResult {
	if issues == nil {
		issues = []string{}
	}
	return CheckResult{
		Name:   name,
		Passed: len(issues) == 0,
		Issues: issues,
	}
}
