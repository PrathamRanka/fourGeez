package stacks

import (
	"testing"
)

// TestServiceDetectsDocumentedStacks verifies exact deterministic evidence rules.
func TestServiceDetectsDocumentedStacks(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		files     map[string]string
		wantStack Stack
		wantTier  SupportTier
	}{
		{name: "Next.js", files: nodeManifest("next", "react"), wantStack: StackNextJS, wantTier: SupportTierMaintained},
		{name: "React Vite", files: nodeManifest("react", "vite"), wantStack: StackReactVite, wantTier: SupportTierMaintained},
		{name: "Remix", files: nodeManifest("@remix-run/react"), wantStack: StackRemix, wantTier: SupportTierMaintained},
		{name: "Nuxt", files: nodeManifest("nuxt"), wantStack: StackNuxt, wantTier: SupportTierMaintained},
		{name: "SvelteKit", files: nodeManifest("@sveltejs/kit"), wantStack: StackSvelteKit, wantTier: SupportTierMaintained},
		{name: "Astro", files: nodeManifest("astro"), wantStack: StackAstro, wantTier: SupportTierMaintained},
		{name: "Express", files: nodeManifest("express"), wantStack: StackExpress, wantTier: SupportTierMaintained},
		{name: "Fastify", files: nodeManifest("fastify"), wantStack: StackFastify, wantTier: SupportTierMaintained},
		{name: "NestJS", files: nodeManifest("@nestjs/core"), wantStack: StackNestJS, wantTier: SupportTierMaintained},
		{name: "Go net http", files: map[string]string{"go.mod": "module seller\n", "main.go": "package main\nimport \"net/http\"\n"}, wantStack: StackGoNetHTTP, wantTier: SupportTierMaintained},
		{name: "Gin", files: map[string]string{"go.mod": "require github.com/gin-gonic/gin v1.10.0\n"}, wantStack: StackGin, wantTier: SupportTierPlanned},
		{name: "Echo", files: map[string]string{"go.mod": "require github.com/labstack/echo/v4 v4.13.4\n"}, wantStack: StackEcho, wantTier: SupportTierPlanned},
		{name: "Fiber", files: map[string]string{"go.mod": "require github.com/gofiber/fiber/v2 v2.52.9\n"}, wantStack: StackFiber, wantTier: SupportTierPlanned},
		{name: "FastAPI", files: map[string]string{"requirements.txt": "fastapi==0.116.1\n"}, wantStack: StackFastAPI, wantTier: SupportTierMaintained},
		{name: "Starlette", files: map[string]string{"pyproject.toml": "dependencies = [\"starlette==0.47.3\"]\n"}, wantStack: StackStarlette, wantTier: SupportTierMaintained},
		{name: "Flask", files: map[string]string{"requirements.txt": "Flask==3.1.2\n"}, wantStack: StackFlask, wantTier: SupportTierPlanned},
		{name: "Django", files: map[string]string{"requirements.txt": "Django==5.2.6\n"}, wantStack: StackDjango, wantTier: SupportTierPlanned},
		{name: "ASP.NET Core", files: map[string]string{"Seller.csproj": `<Project Sdk="Microsoft.NET.Sdk.Web"></Project>`}, wantStack: StackASPNetCore, wantTier: SupportTierUnsupported},
		{name: "Spring Boot", files: map[string]string{"pom.xml": `<artifactId>spring-boot-starter-web</artifactId>`}, wantStack: StackSpringBoot, wantTier: SupportTierUnsupported},
		{name: "Rails", files: map[string]string{"Gemfile": `gem "rails", "8.0.2"`}, wantStack: StackRails, wantTier: SupportTierUnsupported},
		{name: "Laravel", files: map[string]string{"composer.json": `{"require":{"laravel/framework":"12.0.0"}}`}, wantStack: StackLaravel, wantTier: SupportTierUnsupported},
	}

	service := NewService()
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			detections, err := service.Detect(testCase.files)
			if err != nil {
				t.Fatal(err)
			}
			if len(detections) != 1 || detections[0].Stack != testCase.wantStack || detections[0].Tier != testCase.wantTier {
				t.Fatalf("detections = %#v", detections)
			}
		})
	}
}

// TestServiceUsesSpecificFrameworkPrecedence verifies generic markers do not duplicate stacks.
func TestServiceUsesSpecificFrameworkPrecedence(t *testing.T) {
	t.Parallel()

	service := NewService()
	detections, err := service.Detect(map[string]string{
		"package.json": `{"dependencies":{"next":"16.0.0","react":"19.0.0","vite":"7.0.0","express":"5.0.0"}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(detections) != 2 || detections[0].Stack != StackNextJS || detections[1].Stack != StackExpress {
		t.Fatalf("detections = %#v", detections)
	}
}

// TestServiceRejectsUnboundedOrMalformedEvidence verifies the repository-input boundary.
func TestServiceRejectsUnboundedOrMalformedEvidence(t *testing.T) {
	t.Parallel()

	service := NewService()
	if _, err := service.Detect(map[string]string{"package.json": `{`}); err == nil {
		t.Fatal("Detect() accepted malformed package.json")
	}
	files := make(map[string]string)
	for index := 0; index <= MaximumEvidenceFiles; index++ {
		files[string(rune('a'+index%26))+string(rune('0'+index%10))] = "content"
	}
	if _, err := service.Detect(files); err == nil {
		t.Fatal("Detect() accepted too many evidence files")
	}
}

// TestMatrixLabelsEveryDeclaredStack verifies explicit support advertising.
func TestMatrixLabelsEveryDeclaredStack(t *testing.T) {
	t.Parallel()

	matrix := NewService().Matrix()
	if len(matrix) != 21 {
		t.Fatalf("matrix length = %d, want 21", len(matrix))
	}
	for _, entry := range matrix {
		if entry.Stack == "" || entry.Tier == "" || entry.DisplayName == "" {
			t.Fatalf("incomplete matrix entry = %#v", entry)
		}
	}
}

// nodeManifest creates one exact dependency manifest fixture.
func nodeManifest(packages ...string) map[string]string {
	dependencies := ""
	for index, packageName := range packages {
		if index > 0 {
			dependencies += ","
		}
		dependencies += `"` + packageName + `":"1.0.0"`
	}
	return map[string]string{
		"package.json": `{"dependencies":{` + dependencies + `}}`,
	}
}
