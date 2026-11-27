# portal-public-config

Dual-form microservice serving public portal configuration (retired sandbox endpoint and public overview fallback).

## Forms

- **Form B (Primary)**: Cloudflare Worker (`workers/portal-public-config-edge`, TypeScript + Hono) deployed to `portal-public-config-edge`.
- **Form A (Cold-standby fallback)**: Minimal Go 1.25 `net/http` service (`cmd/portal-public-config`), packaged as a distroless container image on GHCR (`ghcr.io/ai-workspace-services/portal-public-config`), scaled to `replicas: 0` in K3s.

## Endpoints

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/healthz` | Liveness probe returning runtime identity |
| `GET` | `/readyz` | Readiness probe |
| `GET` | `/api/internal/sandbox/guest` | Returns `410 Gone` (`sandbox_retired`); checks `INTERNAL_SERVICE_TOKEN` when configured |
| `GET` | `/api/internal/public-overview` | Returns public overview user count fallback; checks `INTERNAL_SERVICE_TOKEN` when configured |
