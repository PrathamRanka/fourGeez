// Package health exposes process liveness and dependency-aware readiness.
package health

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
)

const (
	// LivePath reports whether the HTTP process can serve requests.
	LivePath = "/health/live"
	// ReadyPath reports whether transaction-critical dependencies are available.
	ReadyPath = "/health/ready"
)

// Status is the aggregate health state.
type Status string

const (
	StatusOK       Status = "ok"
	StatusLive     Status = "live"
	StatusReady    Status = "ready"
	StatusNotReady Status = "not_ready"
)

// DependencyStatus is a non-sensitive dependency result.
type DependencyStatus string

const (
	DependencyReady       DependencyStatus = "ready"
	DependencyUnavailable DependencyStatus = "unavailable"
)

// Dependency is one transaction-critical readiness boundary.
type Dependency struct {
	Name  string
	Check func(context.Context) error
}

// DependencyResult intentionally excludes underlying error text and secrets.
type DependencyResult struct {
	Name   string           `json:"name"`
	Status DependencyStatus `json:"status"`
}

// Report is returned by liveness and readiness endpoints.
type Report struct {
	Status       Status             `json:"status"`
	Dependencies []DependencyResult `json:"dependencies,omitempty"`
}

// Controller evaluates configured dependencies within a bounded request.
type Controller struct {
	dependencies []Dependency
	timeout      time.Duration
}

// NewController validates a complete, uniquely named dependency set.
func NewController(dependencies []Dependency, timeout time.Duration) (*Controller, error) {
	if len(dependencies) == 0 {
		return nil, errors.New("at least one readiness dependency is required")
	}
	if timeout <= 0 {
		return nil, errors.New("readiness timeout must be positive")
	}
	names := make(map[string]struct{}, len(dependencies))
	for _, dependency := range dependencies {
		name := strings.TrimSpace(dependency.Name)
		if name == "" || dependency.Check == nil {
			return nil, errors.New("readiness dependencies require a name and check")
		}
		if _, exists := names[name]; exists {
			return nil, errors.New("readiness dependency names must be unique")
		}
		names[name] = struct{}{}
	}
	return &Controller{
		dependencies: append([]Dependency(nil), dependencies...),
		timeout:      timeout,
	}, nil
}

// RegisterRoutes registers liveness, readiness, and the legacy health alias.
func (controller *Controller) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+LivePath, controller.live)
	mux.HandleFunc("GET "+ReadyPath, controller.ready)
	mux.HandleFunc("GET /health", controller.legacy)
}

func (*Controller) legacy(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	_ = api.WriteJSON(response, http.StatusOK, Report{Status: StatusOK})
}

func (*Controller) live(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	_ = api.WriteJSON(response, http.StatusOK, Report{Status: StatusLive})
}

func (controller *Controller) ready(response http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), controller.timeout)
	defer cancel()

	results := make([]DependencyResult, len(controller.dependencies))
	var waitGroup sync.WaitGroup
	for index, dependency := range controller.dependencies {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			status := DependencyReady
			completed := make(chan error, 1)
			go func() {
				completed <- dependency.Check(ctx)
			}()
			select {
			case err := <-completed:
				if err != nil {
					status = DependencyUnavailable
				}
			case <-ctx.Done():
				status = DependencyUnavailable
			}
			results[index] = DependencyResult{Name: dependency.Name, Status: status}
		}()
	}
	waitGroup.Wait()

	report := Report{Status: StatusReady, Dependencies: results}
	statusCode := http.StatusOK
	for _, result := range results {
		if result.Status == DependencyUnavailable {
			report.Status = StatusNotReady
			statusCode = http.StatusServiceUnavailable
			break
		}
	}
	response.Header().Set("Cache-Control", "no-store")
	_ = api.WriteJSON(response, statusCode, report)
}
