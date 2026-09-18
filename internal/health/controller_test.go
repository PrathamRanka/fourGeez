package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestControllerKeepsLivenessIndependentFromDependencies verifies restart semantics.
func TestControllerKeepsLivenessIndependentFromDependencies(t *testing.T) {
	t.Parallel()

	controller, err := NewController([]Dependency{
		{Name: "persistence", Check: func(context.Context) error { return errors.New("offline") }},
	}, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, LivePath, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("liveness status = %d, want 200", response.Code)
	}
	assertHealthStatus(t, response, StatusLive)
}

func TestControllerPreservesLegacyHealthContract(t *testing.T) {
	t.Parallel()

	controller, err := NewController([]Dependency{
		{Name: "persistence", Check: func(context.Context) error { return errors.New("offline") }},
	}, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("legacy health status = %d, want 200", response.Code)
	}
	assertHealthStatus(t, response, StatusOK)
}

// TestControllerReportsReadyOnlyWhenEveryDependencySucceeds verifies fail-closed readiness.
func TestControllerReportsReadyOnlyWhenEveryDependencySucceeds(t *testing.T) {
	t.Parallel()

	controller, err := NewController([]Dependency{
		{Name: "persistence", Check: func(context.Context) error { return nil }},
		{Name: "signing", Check: func(context.Context) error { return nil }},
		{Name: "payment", Check: func(context.Context) error { return nil }},
		{Name: "seller_forwarding", Check: func(context.Context) error { return nil }},
		{Name: "websocket", Check: func(context.Context) error { return nil }},
	}, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, ReadyPath, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("readiness status = %d, body = %s", response.Code, response.Body.String())
	}
	report := decodeHealthReport(t, response)
	if report.Status != StatusReady || len(report.Dependencies) != 5 {
		t.Fatalf("readiness report = %#v", report)
	}
	for _, dependency := range report.Dependencies {
		if dependency.Status != DependencyReady {
			t.Fatalf("dependency = %#v", dependency)
		}
	}
}

// TestControllerReturns503WithoutLeakingDependencyErrors verifies safe failure output.
func TestControllerReturns503WithoutLeakingDependencyErrors(t *testing.T) {
	t.Parallel()

	controller, err := NewController([]Dependency{
		{Name: "persistence", Check: func(context.Context) error { return nil }},
		{Name: "payment", Check: func(context.Context) error {
			return errors.New("Authorization: Bearer secret-payment-token")
		}},
		{Name: "seller_forwarding", Check: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}},
	}, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, ReadyPath, nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness status = %d, want 503", response.Code)
	}
	report := decodeHealthReport(t, response)
	if report.Status != StatusNotReady {
		t.Fatalf("readiness status = %q", report.Status)
	}
	statuses := make(map[string]DependencyStatus)
	for _, dependency := range report.Dependencies {
		statuses[dependency.Name] = dependency.Status
	}
	if statuses["persistence"] != DependencyReady ||
		statuses["payment"] != DependencyUnavailable ||
		statuses["seller_forwarding"] != DependencyUnavailable {
		t.Fatalf("dependency statuses = %#v", statuses)
	}
	if strings.Contains(response.Body.String(), "secret-payment-token") ||
		strings.Contains(response.Body.String(), "Authorization") {
		t.Fatalf("readiness response leaked dependency error: %s", response.Body.String())
	}
}

// TestNewControllerRejectsInvalidDependencyConfiguration verifies startup validation.
func TestNewControllerRejectsInvalidDependencyConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		dependencies []Dependency
		timeout      time.Duration
	}{
		{name: "missing dependencies", timeout: time.Second},
		{name: "missing name", dependencies: []Dependency{{Check: func(context.Context) error { return nil }}}, timeout: time.Second},
		{name: "missing check", dependencies: []Dependency{{Name: "payment"}}, timeout: time.Second},
		{name: "duplicate name", dependencies: []Dependency{{Name: "payment", Check: func(context.Context) error { return nil }}, {Name: "payment", Check: func(context.Context) error { return nil }}}, timeout: time.Second},
		{name: "invalid timeout", dependencies: []Dependency{{Name: "payment", Check: func(context.Context) error { return nil }}}, timeout: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewController(test.dependencies, test.timeout); err == nil {
				t.Fatal("NewController() error = nil, want validation error")
			}
		})
	}
}

func assertHealthStatus(t *testing.T, response *httptest.ResponseRecorder, status Status) {
	t.Helper()
	report := decodeHealthReport(t, response)
	if report.Status != status {
		t.Fatalf("status = %q, want %q", report.Status, status)
	}
}

func decodeHealthReport(t *testing.T, response *httptest.ResponseRecorder) Report {
	t.Helper()
	var report Report
	if err := json.Unmarshal(response.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	return report
}
