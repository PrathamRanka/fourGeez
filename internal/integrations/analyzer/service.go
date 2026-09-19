package analyzer

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Service performs deterministic analysis without persistence or publication.
type Service struct{}

// NewService creates the stateless repository analyzer.
func NewService() *Service {
	return &Service{}
}

// Analyze validates bounded metadata and proposes supported OpenAPI operations.
func (service *Service) Analyze(request Request) (Result, error) {
	if err := validateManifest(request.Manifest); err != nil {
		return Result{}, err
	}
	if len(request.OpenAPI) == 0 || len(request.OpenAPI) > MaximumOpenAPIBytes {
		return Result{}, errors.New("OpenAPI document must contain 1-524288 bytes")
	}

	var document openAPIDocument
	if err := yaml.Unmarshal([]byte(request.OpenAPI), &document); err != nil {
		return Result{}, fmt.Errorf("invalid OpenAPI document: %w", err)
	}
	if !strings.HasPrefix(document.OpenAPI, "3.") {
		return Result{}, errors.New("OpenAPI version must be 3.x")
	}

	result := Result{
		SchemaVersion: RepositoryManifestVersion,
		ServiceName:   strings.TrimSpace(request.Manifest.ServiceName),
		Framework:     request.Manifest.Framework,
		Proposals:     make([]RouteProposal, 0),
		Rejections:    make([]RouteRejection, 0),
	}
	paths := sortedMapKeys(document.Paths)
	for _, routePath := range paths {
		operations := document.Paths[routePath].operations()
		methods := sortedMapKeys(operations)
		for _, method := range methods {
			operation := operations[method]
			upperMethod := strings.ToUpper(method)
			if upperMethod != "GET" && upperMethod != "POST" {
				result.Rejections = append(result.Rejections, RouteRejection{
					Method: upperMethod,
					Path:   routePath,
					Reason: "method is not supported by paid routes",
				})
				continue
			}
			if strings.ContainsAny(routePath, "{}") {
				result.Rejections = append(result.Rejections, RouteRejection{
					Method: upperMethod,
					Path:   routePath,
					Reason: "templated paths are not supported in V1",
				})
				continue
			}
			if len(result.Proposals) >= MaximumRouteProposals {
				return Result{}, errors.New("OpenAPI document exceeds the 50-proposal limit")
			}
			result.Proposals = append(result.Proposals, RouteProposal{
				Method:      upperMethod,
				Path:        routePath,
				Description: operationDescription(operation, upperMethod, routePath),
				MIMEType:    operationMIMEType(operation),
			})
		}
	}
	return result, nil
}

// validateManifest enforces the small repository-metadata allowlist.
func validateManifest(manifest RepositoryManifest) error {
	if manifest.SchemaVersion != RepositoryManifestVersion {
		return errors.New("unsupported repository manifest version")
	}
	serviceName := strings.TrimSpace(manifest.ServiceName)
	if serviceName == "" || len(serviceName) > 120 {
		return errors.New("serviceName must contain 1-120 characters")
	}
	switch manifest.Framework {
	case FrameworkGo,
		FrameworkNode,
		FrameworkPython,
		FrameworkDotNet,
		FrameworkJava,
		FrameworkRuby,
		FrameworkPHP:
	default:
		return errors.New("framework must be go, node, python, dotnet, java, ruby, or php")
	}
	cleanPath := filepath.ToSlash(filepath.Clean(manifest.OpenAPIPath))
	if cleanPath == "." || filepath.IsAbs(manifest.OpenAPIPath) ||
		strings.HasPrefix(cleanPath, "../") {
		return errors.New("openapiPath must be a relative repository path")
	}
	extension := strings.ToLower(filepath.Ext(cleanPath))
	if extension != ".json" && extension != ".yaml" && extension != ".yml" {
		return errors.New("openapiPath must identify a JSON or YAML file")
	}
	return nil
}

// operationDescription chooses deterministic human-readable operation text.
func operationDescription(
	operation openAPIOperation,
	method string,
	routePath string,
) string {
	if summary := strings.TrimSpace(operation.Summary); summary != "" {
		return summary
	}
	if description := strings.TrimSpace(operation.Description); description != "" {
		return description
	}
	return method + " " + routePath
}

// operationMIMEType chooses the first sorted successful response media type.
func operationMIMEType(operation openAPIOperation) string {
	statusCodes := sortedMapKeys(operation.Responses)
	for _, statusCode := range statusCodes {
		if !strings.HasPrefix(statusCode, "2") {
			continue
		}
		mediaTypes := sortedMapKeys(operation.Responses[statusCode].Content)
		if len(mediaTypes) > 0 {
			return mediaTypes[0]
		}
	}
	return "application/json"
}

// sortedMapKeys returns deterministic lexical map ordering.
func sortedMapKeys[Value any](values map[string]Value) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

type openAPIDocument struct {
	OpenAPI string                     `yaml:"openapi"`
	Paths   map[string]openAPIPathItem `yaml:"paths"`
}

type openAPIPathItem struct {
	Get     *openAPIOperation `yaml:"get"`
	Post    *openAPIOperation `yaml:"post"`
	Put     *openAPIOperation `yaml:"put"`
	Patch   *openAPIOperation `yaml:"patch"`
	Delete  *openAPIOperation `yaml:"delete"`
	Head    *openAPIOperation `yaml:"head"`
	Options *openAPIOperation `yaml:"options"`
	Trace   *openAPIOperation `yaml:"trace"`
}

// operations returns only actual HTTP operations from an OpenAPI Path Item.
func (pathItem openAPIPathItem) operations() map[string]openAPIOperation {
	operations := make(map[string]openAPIOperation)
	for method, operation := range map[string]*openAPIOperation{
		"get":     pathItem.Get,
		"post":    pathItem.Post,
		"put":     pathItem.Put,
		"patch":   pathItem.Patch,
		"delete":  pathItem.Delete,
		"head":    pathItem.Head,
		"options": pathItem.Options,
		"trace":   pathItem.Trace,
	} {
		if operation != nil {
			operations[method] = *operation
		}
	}
	return operations
}

type openAPIOperation struct {
	Summary     string                     `yaml:"summary"`
	Description string                     `yaml:"description"`
	Responses   map[string]openAPIResponse `yaml:"responses"`
}

type openAPIResponse struct {
	Content map[string]map[string]any `yaml:"content"`
}
