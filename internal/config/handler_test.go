package config

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthzAndReadyz(t *testing.T) {
	h := NewHandler(Options{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var health map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
		t.Fatalf("decode healthz: %v", err)
	}
	if health["status"] != "ok" || health["runtime"] != "go-origin" || health["service"] != "portal-public-config" {
		t.Fatalf("unexpected healthz payload: %+v", health)
	}

	req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var ready map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &ready); err != nil {
		t.Fatalf("decode readyz: %v", err)
	}
	if ready["status"] != "ready" || ready["runtime"] != "go-origin" {
		t.Fatalf("unexpected readyz payload: %+v", ready)
	}
}

func TestHomepageVideoRetired(t *testing.T) {
	h := NewHandler(Options{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/homepage-video", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after endpoint retirement, got %d", rec.Code)
	}
}

func TestInternalEndpointsAuthAndResponses(t *testing.T) {
	fixedNow := time.Date(2026, 11, 27, 13, 0, 0, 0, time.UTC)
	h := NewHandler(Options{
		InternalServiceToken:        "secret-token",
		PublicOverviewUsersFallback: 42,
		Now:                         func() time.Time { return fixedNow },
	})

	// Missing token -> 401
	for _, path := range []string{"/api/internal/sandbox/guest", "/api/internal/public-overview"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s without token: expected 401, got %d", path, rec.Code)
		}
	}

	// Sandbox guest with X-Internal-Service-Token -> 410
	req := httptest.NewRequest(http.MethodGet, "/api/internal/sandbox/guest", nil)
	req.Header.Set("X-Internal-Service-Token", "secret-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d", rec.Code)
	}
	var gone map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &gone); err != nil {
		t.Fatalf("decode 410: %v", err)
	}
	if gone["error"] != "sandbox_retired" {
		t.Fatalf("unexpected 410 payload: %+v", gone)
	}

	// Public overview with Authorization: Bearer -> 200
	req = httptest.NewRequest(http.MethodGet, "/api/internal/public-overview", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var overview struct {
		RegisteredUsers int    `json:"registeredUsers"`
		UpdatedAt       string `json:"updatedAt"`
		Source          string `json:"source"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &overview); err != nil {
		t.Fatalf("decode overview: %v", err)
	}
	if overview.RegisteredUsers != 42 || overview.UpdatedAt != "2026-11-27T13:00:00Z" || overview.Source != "go-origin" {
		t.Fatalf("unexpected overview payload: %+v", overview)
	}
}

func TestInternalEndpointsWithoutConfiguredToken(t *testing.T) {
	h := NewHandler(Options{})

	req := httptest.NewRequest(http.MethodGet, "/api/internal/sandbox/guest", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusGone {
		t.Fatalf("expected 410 when token unset, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/internal/public-overview", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when token unset, got %d", rec.Code)
	}
}
