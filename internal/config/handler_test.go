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

func TestHomepageVideoResolution(t *testing.T) {
	h := NewHandler(Options{})

	tests := []struct {
		name           string
		host           string
		headers        map[string]string
		wantDomain     string
		wantVideoURL   string
	}{
		{
			name:         "default host",
			host:         "www.svc.plus",
			wantDomain:   "",
			wantVideoURL: "https://www.youtube.com/watch?v=UW6DY6HQnmo",
		},
		{
			name:         "cn override via X-Forwarded-Host with port and uppercase",
			host:         "www.svc.plus",
			headers:      map[string]string{"X-Forwarded-Host": "CN-WWW.SVC.PLUS:443, edge.internal"},
			wantDomain:   "cn-www.svc.plus",
			wantVideoURL: "https://www.bilibili.com/video/BV12DwazxEkL/?spm_id_from=333.1387.homepage.video_card.click&vd_source=e14d146f9a815c7d11e1a1fc23565ffd",
		},
		{
			name:         "cn override via X-Original-Host",
			host:         "www.svc.plus",
			headers:      map[string]string{"X-Original-Host": "cn-www.svc.plus"},
			wantDomain:   "cn-www.svc.plus",
			wantVideoURL: "https://www.bilibili.com/video/BV12DwazxEkL/?spm_id_from=333.1387.homepage.video_card.click&vd_source=e14d146f9a815c7d11e1a1fc23565ffd",
		},
		{
			name:         "cn override via X-Host",
			host:         "www.svc.plus",
			headers:      map[string]string{"X-Host": "cn-www.svc.plus:8443"},
			wantDomain:   "cn-www.svc.plus",
			wantVideoURL: "https://www.bilibili.com/video/BV12DwazxEkL/?spm_id_from=333.1387.homepage.video_card.click&vd_source=e14d146f9a815c7d11e1a1fc23565ffd",
		},
		{
			name:         "cn override via Host",
			host:         "cn-www.svc.plus:443",
			wantDomain:   "cn-www.svc.plus",
			wantVideoURL: "https://www.bilibili.com/video/BV12DwazxEkL/?spm_id_from=333.1387.homepage.video_card.click&vd_source=e14d146f9a815c7d11e1a1fc23565ffd",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/auth/homepage-video", nil)
			req.Host = tc.host
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", rec.Code)
			}
			if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=300" {
				t.Fatalf("unexpected Cache-Control: %q", cc)
			}
			var body struct {
				Resolved HomepageVideoEntry `json:"resolved"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Resolved.Domain != tc.wantDomain || body.Resolved.VideoURL != tc.wantVideoURL || body.Resolved.PosterURL != "" {
				t.Fatalf("unexpected resolved entry: %+v", body.Resolved)
			}
		})
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
