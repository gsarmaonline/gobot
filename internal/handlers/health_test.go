package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthCheck(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	HealthCheck(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp["status"])
	}

	if resp["service"] != "gobot" {
		t.Errorf("expected service 'gobot', got '%s'", resp["service"])
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", ct)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]string{"key": "value"})

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["key"] != "value" {
		t.Errorf("expected key 'value', got '%s'", resp["key"])
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "something went wrong")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["error"] != "something went wrong" {
		t.Errorf("expected error message 'something went wrong', got '%s'", resp["error"])
	}
}
