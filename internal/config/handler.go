package config

import (
	"crypto/subtle"
	"encoding/json"
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

// NewHandler returns an http.Handler serving all portal-public-config endpoints.
func NewHandler(opts Options) http.Handler {
	if opts.Now == nil {
		opts.Now = time.Now
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /readyz", handleReadyz)
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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
