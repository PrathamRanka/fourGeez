package stacks

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"
	"strings"
)

var stackMatrix = []MatrixEntry{
	{Stack: StackNextJS, DisplayName: "Next.js", Tier: SupportTierMaintained},
	{Stack: StackReactVite, DisplayName: "React/Vite", Tier: SupportTierMaintained},
	{Stack: StackRemix, DisplayName: "Remix", Tier: SupportTierMaintained},
	{Stack: StackNuxt, DisplayName: "Nuxt", Tier: SupportTierMaintained},
	{Stack: StackSvelteKit, DisplayName: "SvelteKit", Tier: SupportTierMaintained},
	{Stack: StackAstro, DisplayName: "Astro", Tier: SupportTierMaintained},
	{Stack: StackExpress, DisplayName: "Express", Tier: SupportTierMaintained},
	{Stack: StackFastify, DisplayName: "Fastify", Tier: SupportTierMaintained},
	{Stack: StackNestJS, DisplayName: "NestJS", Tier: SupportTierMaintained},
	{Stack: StackGoNetHTTP, DisplayName: "Go net/http", Tier: SupportTierMaintained},
	{Stack: StackGin, DisplayName: "Gin", Tier: SupportTierMaintained},
	{Stack: StackEcho, DisplayName: "Echo", Tier: SupportTierMaintained},
	{Stack: StackFiber, DisplayName: "Fiber", Tier: SupportTierMaintained},
	{Stack: StackFastAPI, DisplayName: "FastAPI", Tier: SupportTierMaintained},
	{Stack: StackStarlette, DisplayName: "Starlette", Tier: SupportTierMaintained},
	{Stack: StackFlask, DisplayName: "Flask", Tier: SupportTierMaintained},
	{Stack: StackDjango, DisplayName: "Django", Tier: SupportTierMaintained},
	{Stack: StackASPNetCore, DisplayName: "ASP.NET Core", Tier: SupportTierMaintained},
	{Stack: StackSpringBoot, DisplayName: "Spring Boot", Tier: SupportTierMaintained},
	{Stack: StackRails, DisplayName: "Rails", Tier: SupportTierMaintained},
	{Stack: StackLaravel, DisplayName: "Laravel", Tier: SupportTierMaintained},
}

// Service performs deterministic stack detection over bounded repository evidence.
type Service struct{}

// NewService creates the stateless stack detector.
func NewService() *Service {
	return &Service{}
}

// Matrix returns an isolated deterministic support matrix.
func (service *Service) Matrix() []MatrixEntry {
	return append([]MatrixEntry(nil), stackMatrix...)
}

// Detect returns every evidenced stack in support-matrix order.
func (service *Service) Detect(files map[string]string) ([]Detection, error) {
	normalizedFiles, err := normalizeEvidence(files)
	if err != nil {
		return nil, err
	}
	nodeDependencies, err := readNodeDependencies(normalizedFiles)
	if err != nil {
		return nil, err
	}
	detectedEvidence, err := detectEvidence(normalizedFiles, nodeDependencies)
	if err != nil {
		return nil, err
	}
	detections := make([]Detection, 0, len(detectedEvidence))
	for _, entry := range stackMatrix {
		evidence := detectedEvidence[entry.Stack]
		if len(evidence) == 0 {
			continue
		}
		sort.Strings(evidence)
		detections = append(detections, Detection{
			Stack:    entry.Stack,
			Tier:     entry.Tier,
			Evidence: evidence,
		})
	}
	return detections, nil
}

// normalizeEvidence validates repository-relative paths and total input size.
func normalizeEvidence(files map[string]string) (map[string]string, error) {
	if len(files) == 0 {
		return nil, errors.New("repository evidence is required")
	}
	if len(files) > MaximumEvidenceFiles {
		return nil, errors.New("repository evidence exceeds the 100-file limit")
	}
	normalized := make(map[string]string, len(files))
	totalBytes := 0
	for path, content := range files {
		cleanPath := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
		if cleanPath == "." || filepath.IsAbs(path) || strings.HasPrefix(cleanPath, "../") || strings.ContainsRune(cleanPath, '\x00') {
			return nil, errors.New("evidence paths must remain inside the repository")
		}
		cleanPath = strings.ToLower(cleanPath)
		if !isAllowedEvidencePath(cleanPath) {
			return nil, errors.New("repository evidence path is not allowlisted")
		}
		totalBytes += len(cleanPath) + len(content)
		if totalBytes > MaximumEvidenceBytes {
			return nil, errors.New("repository evidence exceeds the 1 MiB limit")
		}
		normalized[cleanPath] = content
	}
	return normalized, nil
}

// isAllowedEvidencePath keeps environment files and unrelated source out of MCP input.
func isAllowedEvidencePath(path string) bool {
	baseName := filepath.Base(path)
	return baseName == "package.json" ||
		baseName == "go.mod" ||
		strings.HasSuffix(baseName, ".go") ||
		baseName == "requirements.txt" ||
		baseName == "pyproject.toml" ||
		strings.HasSuffix(baseName, ".csproj") ||
		baseName == "pom.xml" ||
		strings.HasSuffix(baseName, ".gradle") ||
		strings.HasSuffix(baseName, ".gradle.kts") ||
		baseName == "gemfile" ||
		baseName == "composer.json"
}

// readNodeDependencies parses exact dependency names from every package.json.
func readNodeDependencies(files map[string]string) (map[string][]string, error) {
	dependencies := make(map[string][]string)
	for path, content := range files {
		if filepath.Base(path) != "package.json" {
			continue
		}
		var manifest struct {
			Dependencies         map[string]string `json:"dependencies"`
			DevDependencies      map[string]string `json:"devDependencies"`
			PeerDependencies     map[string]string `json:"peerDependencies"`
			OptionalDependencies map[string]string `json:"optionalDependencies"`
		}
		if err := json.Unmarshal([]byte(content), &manifest); err != nil {
			return nil, errors.New("package.json must contain valid JSON")
		}
		for _, dependencySet := range []map[string]string{
			manifest.Dependencies,
			manifest.DevDependencies,
			manifest.PeerDependencies,
			manifest.OptionalDependencies,
		} {
			for packageName := range dependencySet {
				normalizedName := strings.ToLower(packageName)
				dependencies[normalizedName] = append(dependencies[normalizedName], path)
			}
		}
	}
	return dependencies, nil
}

// detectEvidence applies exact package and source rules without guessing.
func detectEvidence(
	files map[string]string,
	nodeDependencies map[string][]string,
) (map[Stack][]string, error) {
	detected := make(map[Stack][]string)
	addNodeDetection(detected, nodeDependencies, StackNextJS, "next")
	addNodeDetection(detected, nodeDependencies, StackRemix, "@remix-run/react")
	addNodeDetection(detected, nodeDependencies, StackNuxt, "nuxt")
	addNodeDetection(detected, nodeDependencies, StackSvelteKit, "@sveltejs/kit")
	addNodeDetection(detected, nodeDependencies, StackAstro, "astro")
	addNodeDetection(detected, nodeDependencies, StackExpress, "express")
	addNodeDetection(detected, nodeDependencies, StackFastify, "fastify")
	addNodeDetection(detected, nodeDependencies, StackNestJS, "@nestjs/core")
	if hasNodeDependency(nodeDependencies, "react") &&
		hasNodeDependency(nodeDependencies, "vite") &&
		!hasNodeMetaframework(detected) {
		detected[StackReactVite] = combinedEvidence(
			nodeDependencies["react"],
			nodeDependencies["vite"],
		)
	}

	detectGoStacks(files, detected)
	detectPythonStacks(files, detected)
	if err := detectExtendedStacks(files, detected); err != nil {
		return nil, err
	}
	return detected, nil
}

// addNodeDetection records one exact Node package match.
func addNodeDetection(
	detected map[Stack][]string,
	dependencies map[string][]string,
	stack Stack,
	packageName string,
) {
	if evidence := dependencies[packageName]; len(evidence) > 0 {
		detected[stack] = combinedEvidence(evidence)
	}
}

// hasNodeDependency reports whether an exact package name is declared.
func hasNodeDependency(dependencies map[string][]string, packageName string) bool {
	return len(dependencies[packageName]) > 0
}

// combinedEvidence merges exact manifest paths without duplicates.
func combinedEvidence(groups ...[]string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, group := range groups {
		for _, path := range group {
			if _, exists := seen[path]; exists {
				continue
			}
			seen[path] = struct{}{}
			result = append(result, path)
		}
	}
	return result
}

// hasNodeMetaframework reports whether a stronger rendering framework matched.
func hasNodeMetaframework(detected map[Stack][]string) bool {
	for _, stack := range []Stack{StackNextJS, StackRemix, StackNuxt, StackSvelteKit, StackAstro} {
		if len(detected[stack]) > 0 {
			return true
		}
	}
	return false
}

// detectGoStacks applies module-first Go framework precedence.
func detectGoStacks(files map[string]string, detected map[Stack][]string) {
	goModules := make(map[string]string)
	for path, content := range files {
		if filepath.Base(path) == "go.mod" {
			goModules[path] = content
		}
	}
	if len(goModules) == 0 {
		return
	}
	goFrameworkDetected := false
	for path, goModule := range goModules {
		lowerModule := strings.ToLower(goModule)
		for moduleName, stack := range map[string]Stack{
			"github.com/gin-gonic/gin": StackGin,
			"github.com/labstack/echo": StackEcho,
			"github.com/gofiber/fiber": StackFiber,
		} {
			if strings.Contains(lowerModule, moduleName) {
				detected[stack] = append(detected[stack], path)
				goFrameworkDetected = true
			}
		}
	}
	if goFrameworkDetected {
		return
	}
	for path, content := range files {
		if strings.HasSuffix(path, ".go") && strings.Contains(content, `"net/http"`) {
			modulePaths := make([]string, 0, len(goModules)+1)
			for modulePath := range goModules {
				modulePaths = append(modulePaths, modulePath)
			}
			detected[StackGoNetHTTP] = append(modulePaths, path)
			return
		}
	}
}

// detectPythonStacks applies exact normalized dependency matches.
func detectPythonStacks(files map[string]string, detected map[Stack][]string) {
	dependencies := pythonDependencies(files)
	if _, exists := dependencies["fastapi"]; exists {
		detected[StackFastAPI] = dependencies["fastapi"]
	} else if _, exists := dependencies["starlette"]; exists {
		detected[StackStarlette] = dependencies["starlette"]
	}
	if _, exists := dependencies["flask"]; exists {
		detected[StackFlask] = dependencies["flask"]
	}
	if _, exists := dependencies["django"]; exists {
		detected[StackDjango] = dependencies["django"]
	}
}

// pythonDependencies extracts package names from supported Python manifests.
func pythonDependencies(files map[string]string) map[string][]string {
	result := make(map[string][]string)
	for path, content := range files {
		baseName := filepath.Base(path)
		if baseName != "requirements.txt" && baseName != "pyproject.toml" {
			continue
		}
		lowerContent := strings.ToLower(content)
		for _, packageName := range []string{"fastapi", "starlette", "flask", "django"} {
			if manifestContainsPackage(lowerContent, packageName) {
				result[packageName] = append(result[packageName], path)
			}
		}
	}
	return result
}

// manifestContainsPackage matches a package token without substring aliases.
func manifestContainsPackage(content string, packageName string) bool {
	for _, separator := range []string{"\n", "\r", "\t", " ", "\"", "'", ",", "[", "]", "{", "}"} {
		content = strings.ReplaceAll(content, separator, "|")
	}
	for _, token := range strings.Split(content, "|") {
		candidate := strings.TrimSpace(token)
		candidate = strings.SplitN(candidate, "#", 2)[0]
		candidate = strings.TrimSpace(candidate)
		for _, versionSeparator := range []string{"==", ">=", "<=", "~=", "!=", ">", "<", "="} {
			candidate = strings.SplitN(candidate, versionSeparator, 2)[0]
		}
		candidate = strings.SplitN(candidate, "[", 2)[0]
		if strings.TrimSpace(candidate) == packageName {
			return true
		}
	}
	return false
}

// detectExtendedStacks finds maintained ecosystems outside Node, Go, and Python.
func detectExtendedStacks(files map[string]string, detected map[Stack][]string) error {
	for path, content := range files {
		lowerContent := strings.ToLower(content)
		switch {
		case strings.HasSuffix(path, ".csproj") &&
			(strings.Contains(lowerContent, "microsoft.net.sdk.web") || strings.Contains(lowerContent, "microsoft.aspnetcore")):
			detected[StackASPNetCore] = []string{path}
		case (filepath.Base(path) == "pom.xml" || strings.HasSuffix(path, ".gradle") || strings.HasSuffix(path, ".gradle.kts")) && strings.Contains(lowerContent, "spring-boot"):
			detected[StackSpringBoot] = []string{path}
		case filepath.Base(path) == "gemfile" && manifestContainsPackage(lowerContent, "rails"):
			detected[StackRails] = []string{path}
		case filepath.Base(path) == "composer.json":
			var manifest struct {
				Require map[string]string `json:"require"`
			}
			if err := json.Unmarshal([]byte(content), &manifest); err != nil {
				return errors.New("composer.json must contain valid JSON")
			}
			if _, exists := manifest.Require["laravel/framework"]; exists {
				detected[StackLaravel] = []string{path}
			}
		}
	}
	return nil
}
