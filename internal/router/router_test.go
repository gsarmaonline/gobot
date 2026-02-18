package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gsarmaonline/gobot/internal/handlers"
	"github.com/gsarmaonline/gobot/internal/models"
)

func TestCORSHeaders(t *testing.T) {
	// Create a minimal handler to test CORS middleware
	handler := withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test normal request
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("expected CORS origin '*', got '%s'", origin)
	}

	// Test preflight OPTIONS request
	req = httptest.NewRequest("OPTIONS", "/", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS, got %d", w.Code)
	}
}

func TestHealthEndpoint(t *testing.T) {
	// We can't create full stores without a DB, but we can test health endpoint
	// by creating the router with nil stores (health doesn't use them)
	// This will panic if health endpoint tries to use stores, which it shouldn't

	// Create a simple mux with just health
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", handlers.HealthCheck)

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 from health endpoint, got %d", w.Code)
	}
}

// TestRouterCreation verifies that New() doesn't panic
func TestRouterCreation(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("router creation panicked: %v", r)
		}
	}()

	// Pass nil DB-backed stores — this tests that the router setup itself works
	// (the handlers won't be called, so nil DB is fine)
	orgHandler := handlers.NewOrganizationHandler(models.NewOrganizationStore(nil))
	agentHandler := handlers.NewAgentHandler(models.NewAgentStore(nil))
	deptHandler := handlers.NewDepartmentHandler(models.NewDepartmentStore(nil))
	hierarchyHandler := handlers.NewHierarchyHandler(models.NewHierarchyStore(nil))

	r := New(orgHandler, agentHandler, deptHandler, hierarchyHandler)
	if r == nil {
		t.Error("expected non-nil router")
	}
}
