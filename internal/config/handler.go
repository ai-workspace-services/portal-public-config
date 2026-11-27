package config

import (
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	RuntimeGoOrigin = "go-origin"
	ServiceName     = "portal-public-config"
)

// HomepageVideoEntry represents a resolved homepage video configuration.
type HomepageVideoEntry struct {
	Domain    string `json:"domain"`
	VideoURL  string `json:"videoUrl"`
	PosterURL string `json:"posterUrl"`
}

// Options configures the HTTP handler.
type Options struct {
	InternalServiceToken        string
	PublicOverviewUsersFallback int
	Now                         func() time.Time
}

// LoadOptionsFromEnv constructs Options from environment variables.
func LoadOptionsFromEnv() Options {
	fallback := 0
	if raw := strings.TrimSpace(os.Getenv("PUBLIC_OVERVIEW_USERS_FALLBACK")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			fallback = parsed
		}
	}
	return Options{
		InternalServiceToken:        strings.TrimSpace(os.Getenv("INTERNAL_SERVICE_TOKEN")),
		PublicOverviewUsersFallback: fallback,
		Now:                         time.Now,
	}
}

var defaultHomepageVideo = HomepageVideoEntry{
	Domain:    "",
	VideoURL:  "https://www.youtube.com/watch?v=UW6DY6HQnmo",
	PosterURL: "",
}

var homepageVideoOverrides = map[string]HomepageVideoEntry{
	"cn-www.svc.plus": {
		Domain:    "cn-www.svc.plus",
		VideoURL:  "https://www.bilibili.com/video/BV12DwazxEkL/?spm_id_from=333.1387.homepage.video_card.click&vd_source=e14d146f9a815c7d11e1a1fc23565ffd",
		PosterURL: "",
	},
}

// NewHandler returns an http.Handler serving all portal-public-config endpoints.
func NewHandler(opts Options) http.Handler {
	if opts.Now == nil {
		opts.Now = time.Now
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /readyz", handleReadyz)
	mux.HandleFunc("GET /api/auth/homepage-video", handleHomepageVideo)
	mux.HandleFunc("GET /api/internal/sandbox/guest", withInternalAuth(opts.InternalServiceToken, handleSandboxGuest))
	mux.HandleFunc("GET /api/internal/public-overview", withInternalAuth(opts.InternalServiceToken, func(w http.ResponseWriter, r *http.Request) {
		handlePublicOverview(w, r, opts)
	}))

	return mux
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"runtime": RuntimeGoOrigin,
		"service": ServiceName,
	})
}

func handleReadyz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ready",
		"runtime": RuntimeGoOrigin,
	})
}

func handleHomepageVideo(w http.ResponseWriter, r *http.Request) {
	host := resolveRequestHost(r)
	entry := defaultHomepageVideo
	if override, ok := homepageVideoOverrides[host]; ok {
		entry = override
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("X-Portal-Config-Runtime", RuntimeGoOrigin)
	writeJSON(w, http.StatusOK, map[string]HomepageVideoEntry{
		"resolved": entry,
	})
}

func handleSandboxGuest(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusGone, map[string]string{
		"error":   "sandbox_retired",
		"message": "sandbox demo account has been retired",
	})
}

func handlePublicOverview(w http.ResponseWriter, _ *http.Request, opts Options) {
	writeJSON(w, http.StatusOK, map[string]any{
		"registeredUsers": opts.PublicOverviewUsersFallback,
		"updatedAt":       opts.Now().UTC().Format(time.RFC3339),
		"source":          RuntimeGoOrigin,
	})
}

func withInternalAuth(expectedToken string, next http.HandlerFunc) http.HandlerFunc {
	expected := strings.TrimSpace(expectedToken)
	return func(w http.ResponseWriter, r *http.Request) {
		if expected != "" {
			provided := extractInternalToken(r)
			if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
				writeJSON(w, http.StatusUnauthorized, map[string]string{
					"error":   "unauthorized",
					"message": "missing or invalid internal service token",
				})
				return
			}
		}
		next(w, r)
	}
}

func extractInternalToken(r *http.Request) string {
	if tok := strings.TrimSpace(r.Header.Get("X-Internal-Service-Token")); tok != "" {
		return tok
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
}

func resolveRequestHost(r *http.Request) string {
	candidates := []string{
		r.Header.Get("X-Forwarded-Host"),
		r.Header.Get("X-Original-Host"),
		r.Header.Get("X-Host"),
		r.Host,
	}
	for _, raw := range candidates {
		if normalized := normalizeHost(raw); normalized != "" {
			return normalized
		}
	}
	return ""
}

func normalizeHost(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if idx := strings.IndexByte(s, ','); idx >= 0 {
		s = strings.TrimSpace(s[:idx])
	}
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	} else if strings.Count(s, ":") == 1 {
		s = s[:strings.IndexByte(s, ':')]
	}
	s = strings.Trim(s, "[]")
	return strings.ToLower(strings.TrimSpace(s))
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
