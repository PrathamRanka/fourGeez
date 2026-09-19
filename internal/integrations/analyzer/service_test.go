package analyzer

import (
	"strings"
	"testing"
)

// TestServiceProposesSupportedOpenAPIRoutesDeterministically verifies bounded analysis.
func TestServiceProposesSupportedOpenAPIRoutesDeterministically(t *testing.T) {
	t.Parallel()

	service := NewService()
	request := Request{
		Manifest: RepositoryManifest{
			SchemaVersion: "agentpay.repository.v1",
			ServiceName:   "Research API",
			Framework:     FrameworkGo,
			OpenAPIPath:   "openapi.yaml",
		},
		OpenAPI: `openapi: 3.1.0
info:
  title: Research API
  version: 1.0.0
paths:
  /zeta:
    post:
      summary: Generate zeta
      responses:
        "200":
          content:
            application/json: {}
  /alpha:
    parameters:
      - name: locale
        in: query
    get:
      description: Read alpha
      responses:
        "200":
          content:
            text/plain: {}
  /users/{userId}:
    get:
      summary: Read user
  /admin:
    delete:
      summary: Delete everything
`,
	}

	result, err := service.Analyze(request)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Proposals) != 2 {
		t.Fatalf("proposals = %#v", result.Proposals)
	}
	if result.Proposals[0].Path != "/alpha" ||
		result.Proposals[1].Path != "/zeta" {
		t.Fatalf("proposal order = %#v", result.Proposals)
	}
	if result.Proposals[0].MIMEType != "text/plain" ||
		result.Proposals[1].MIMEType != "application/json" {
		t.Fatalf("proposal MIME types = %#v", result.Proposals)
	}
	if len(result.Rejections) != 2 {
		t.Fatalf("rejections = %#v", result.Rejections)
	}
}

// TestServiceRejectsUnsafeOrOversizedAnalysisInput verifies strict boundaries.
func TestServiceRejectsUnsafeOrOversizedAnalysisInput(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		request Request
	}{
		{
			name: "unsupported manifest version",
			request: Request{
				Manifest: RepositoryManifest{SchemaVersion: "future"},
				OpenAPI:  `{}`,
			},
		},
		{
			name: "unsupported framework",
			request: Request{
				Manifest: RepositoryManifest{
					SchemaVersion: "agentpay.repository.v1",
					ServiceName:   "Unsafe",
					Framework:     "unknown",
					OpenAPIPath:   "openapi.yaml",
				},
				OpenAPI: `{}`,
			},
		},
		{
			name: "oversized OpenAPI",
			request: Request{
				Manifest: RepositoryManifest{
					SchemaVersion: "agentpay.repository.v1",
					ServiceName:   "Large",
					Framework:     FrameworkNode,
					OpenAPIPath:   "openapi.yaml",
				},
				OpenAPI: strings.Repeat("x", MaximumOpenAPIBytes+1),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if _, err := NewService().Analyze(testCase.request); err == nil {
				t.Fatal("Analyze() accepted unsafe input")
			}
		})
	}
}

// TestServiceAcceptsEveryMaintainedLanguageFamily keeps route analysis aligned
// with the exact 21-stack setup matrix.
func TestServiceAcceptsEveryMaintainedLanguageFamily(t *testing.T) {
	t.Parallel()

	for _, framework := range []Framework{
		FrameworkGo,
		FrameworkNode,
		FrameworkPython,
		FrameworkDotNet,
		FrameworkJava,
		FrameworkRuby,
		FrameworkPHP,
	} {
		t.Run(string(framework), func(t *testing.T) {
			t.Parallel()

			result, err := NewService().Analyze(Request{
				Manifest: RepositoryManifest{
					SchemaVersion: RepositoryManifestVersion,
					ServiceName:   "Seller API",
					Framework:     framework,
					OpenAPIPath:   "openapi.yaml",
				},
				OpenAPI: "openapi: 3.1.0\npaths: {}\n",
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Framework != framework {
				t.Fatalf("framework = %q, want %q", result.Framework, framework)
			}
		})
	}
}
