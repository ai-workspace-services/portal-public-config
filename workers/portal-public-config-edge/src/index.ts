import { Hono } from 'hono';

export interface Env {
  INTERNAL_SERVICE_TOKEN?: string;
  PUBLIC_OVERVIEW_USERS_FALLBACK?: string;
}

export interface HomepageVideoEntry {
  domain: string;
  videoUrl: string;
  posterUrl: string;
}

const RUNTIME = 'cloudflare-worker';
const SERVICE = 'portal-public-config';

const DEFAULT_HOMEPAGE_VIDEO: HomepageVideoEntry = {
  domain: '',
  videoUrl: 'https://www.youtube.com/watch?v=UW6DY6HQnmo',
  posterUrl: '',
};

const HOMEPAGE_VIDEO_OVERRIDES: Record<string, HomepageVideoEntry> = {
  'cn-www.svc.plus': {
    domain: 'cn-www.svc.plus',
    videoUrl:
      'https://www.bilibili.com/video/BV12DwazxEkL/?spm_id_from=333.1387.homepage.video_card.click&vd_source=e14d146f9a815c7d11e1a1fc23565ffd',
    posterUrl: '',
  },
};

export function normalizeHost(raw: string | null | undefined): string {
  if (!raw) return '';
  let host = raw.trim();
  if (!host) return '';
  const commaIdx = host.indexOf(',');
  if (commaIdx >= 0) {
    host = host.slice(0, commaIdx).trim();
  }
  if (host.startsWith('[')) {
    const closeBracket = host.indexOf(']');
    if (closeBracket >= 0) {
      host = host.slice(1, closeBracket);
    }
  } else {
    const colonIdx = host.indexOf(':');
    if (colonIdx >= 0 && host.indexOf(':', colonIdx + 1) === -1) {
      host = host.slice(0, colonIdx);
    }
  }
  return host.trim().toLowerCase();
}

export function resolveRequestHost(req: Request): string {
  const candidates = [
    req.headers.get('X-Forwarded-Host'),
    req.headers.get('X-Original-Host'),
    req.headers.get('X-Host'),
    req.headers.get('Host'),
  ];
  for (const candidate of candidates) {
    const normalized = normalizeHost(candidate);
    if (normalized) return normalized;
  }
  try {
    return normalizeHost(new URL(req.url).host);
  } catch {
    return '';
  }
}

function extractInternalToken(req: Request): string {
  const direct = req.headers.get('X-Internal-Service-Token')?.trim();
  if (direct) return direct;
  const auth = req.headers.get('Authorization')?.trim() ?? '';
  if (auth.length > 7 && auth.slice(0, 7).toLowerCase() === 'bearer ') {
    return auth.slice(7).trim();
  }
  return '';
}

function constantTimeEqual(a: string, b: string): boolean {
  if (a.length !== b.length) return false;
  let diff = 0;
  for (let i = 0; i < a.length; i += 1) {
    diff |= a.charCodeAt(i) ^ b.charCodeAt(i);
  }
  return diff === 0;
}

function isAuthorizedInternal(req: Request, env: Env | undefined): boolean {
  const expected = env?.INTERNAL_SERVICE_TOKEN?.trim() ?? '';
  if (!expected) return true;
  const provided = extractInternalToken(req);
  return constantTimeEqual(provided, expected);
}

function parseUsersFallback(raw: string | undefined): number {
  if (!raw) return 0;
  const parsed = Number.parseInt(raw.trim(), 10);
  if (!Number.isFinite(parsed) || parsed < 0) return 0;
  return parsed;
}

export function createApp(): Hono<{ Bindings: Env }> {
  const app = new Hono<{ Bindings: Env }>();

  app.get('/healthz', (c) =>
    c.json({
      status: 'ok',
      runtime: RUNTIME,
      service: SERVICE,
    }),
  );

  app.get('/readyz', (c) =>
    c.json({
      status: 'ready',
      runtime: RUNTIME,
    }),
  );

  app.get('/api/auth/homepage-video', (c) => {
    const host = resolveRequestHost(c.req.raw);
    const resolved = HOMEPAGE_VIDEO_OVERRIDES[host] ?? DEFAULT_HOMEPAGE_VIDEO;
    c.header('Cache-Control', 'public, max-age=300');
    c.header('X-Portal-Config-Runtime', RUNTIME);
    return c.json({ resolved });
  });

  app.get('/api/internal/sandbox/guest', (c) => {
    if (!isAuthorizedInternal(c.req.raw, c.env)) {
      return c.json(
        {
          error: 'unauthorized',
          message: 'missing or invalid internal service token',
        },
        401,
      );
    }
    return c.json(
      {
        error: 'sandbox_retired',
        message: 'sandbox demo account has been retired',
      },
      410,
    );
  });

  app.get('/api/internal/public-overview', (c) => {
    if (!isAuthorizedInternal(c.req.raw, c.env)) {
      return c.json(
        {
          error: 'unauthorized',
          message: 'missing or invalid internal service token',
        },
        401,
      );
    }
    return c.json({
      registeredUsers: parseUsersFallback(c.env?.PUBLIC_OVERVIEW_USERS_FALLBACK),
      updatedAt: new Date().toISOString(),
      source: RUNTIME,
    });
  });

  return app;
}

const app = createApp();

export default app;
