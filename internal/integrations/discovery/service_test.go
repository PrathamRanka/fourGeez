package discovery

import (
	"strings"
	"testing"
)

// TestServiceValidatesCompleteStorefrontArtifacts verifies every deterministic check.
func TestServiceValidatesCompleteStorefrontArtifacts(t *testing.T) {
	t.Parallel()

	result, err := NewService().Validate(validArtifactRequest())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid || len(result.Checks) != 10 {
		t.Fatalf("result = %#v", result)
	}
	for _, check := range result.Checks {
		if !check.Passed {
			t.Fatalf("failed check = %#v", check)
		}
	}
}

// TestServiceReportsEachArtifactFailure verifies named checks without ranking claims.
func TestServiceReportsEachArtifactFailure(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		checkName CheckName
		mutate    func(*Request)
	}{
		{name: "metadata", checkName: CheckMetadata, mutate: func(request *Request) { request.Pages[0].Title = "Short" }},
		{name: "canonical", checkName: CheckCanonical, mutate: func(request *Request) { request.Pages[0].CanonicalURL = "https://other.example/products/research" }},
		{name: "robots", checkName: CheckRobots, mutate: func(request *Request) { request.RobotsTXT = "User-agent: *\nDisallow: /\n" }},
		{name: "sitemap", checkName: CheckSitemap, mutate: func(request *Request) { request.SitemapXML = `<urlset></urlset>` }},
		{name: "structured data", checkName: CheckStructuredData, mutate: func(request *Request) {
			request.Pages[0].StructuredDataJSON = `{"@type":"Product","name":"Fabricated"}`
		}},
		{name: "semantic content", checkName: CheckSemanticContent, mutate: func(request *Request) { request.Pages[0].HeadingOne = "" }},
		{name: "llms", checkName: CheckLLMSText, mutate: func(request *Request) { request.LLMSText = "# Seller" }},
		{name: "manifest", checkName: CheckManifestConsistency, mutate: func(request *Request) {
			request.ManifestJSON = `{"seller":{"name":"Seller","slug":"seller"},"routes":[]}`
		}},
		{name: "accessibility", checkName: CheckAccessibility, mutate: func(request *Request) { request.Pages[0].UnlabelledControls = 1 }},
		{name: "performance", checkName: CheckPerformance, mutate: func(request *Request) { request.Pages[0].JavaScriptBytes = MaximumJavaScriptBytes + 1 }},
	}

	service := NewService()
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			request := validArtifactRequest()
			testCase.mutate(&request)
			result, err := service.Validate(request)
			if err != nil {
				t.Fatal(err)
			}
			if result.Valid || checkPassed(result, testCase.checkName) {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

// TestServiceRejectsUnboundedArtifacts verifies validation input limits.
func TestServiceRejectsUnboundedArtifacts(t *testing.T) {
	t.Parallel()

	request := validArtifactRequest()
	request.LLMSText = strings.Repeat("x", MaximumArtifactBytes+1)
	if _, err := NewService().Validate(request); err == nil {
		t.Fatal("Validate() accepted oversized artifacts")
	}
}

// validArtifactRequest returns one internally consistent storefront artifact set.
func validArtifactRequest() Request {
	return Request{
		BaseURL: "https://seller.example",
		Pages: []PageArtifact{
			{
				URL:                      "https://seller.example/products/research",
				RoutePath:                "/research",
				Title:                    "Research report API product",
				Description:              "Purchase a focused research report generated from the seller's published API route.",
				CanonicalURL:             "https://seller.example/products/research",
				Robots:                   "index,follow",
				HeadingOne:               "Research report API product",
				VisibleText:              strings.Repeat("Research report details and delivery terms. ", 4),
				Language:                 "en",
				MainLandmarks:            1,
				MissingImageAltCount:     0,
				UnlabelledControls:       0,
				StructuredDataJSON:       `{"@context":"https://schema.org","@type":"Product","name":"Research report API product","description":"Purchase a focused research report generated from the seller's published API route.","url":"https://seller.example/products/research"}`,
				HTMLBytes:                60_000,
				JavaScriptBytes:          120_000,
				CSSBytes:                 30_000,
				BlockingScriptCount:      1,
				LargestContentfulPaintMS: 1_800,
			},
		},
		RobotsTXT:    "User-agent: *\nAllow: /\nSitemap: https://seller.example/sitemap.xml\n",
		SitemapXML:   `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>https://seller.example/products/research</loc></url></urlset>`,
		LLMSText:     "# Seller\n- [Research report API product](https://seller.example/products/research): Purchase a focused research report generated from the seller's published API route.\n",
		ManifestJSON: `{"seller":{"name":"Seller","slug":"seller"},"routes":[{"pathPattern":"/research","description":"Purchase a focused research report generated from the seller's published API route.","enabled":true}]}`,
	}
}

// checkPassed returns one named check outcome.
func checkPassed(result Result, checkName CheckName) bool {
	for _, check := range result.Checks {
		if check.Name == checkName {
			return check.Passed
		}
	}
	return false
}
