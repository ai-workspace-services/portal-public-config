import { Hono } from 'hono';

export interface Env {
  INTERNAL_SERVICE_TOKEN?: string;
  PUBLIC_OVERVIEW_USERS_FALLBACK?: string;
}

const RUNTIME = 'cloudflare-worker';
const SERVICE = 'portal-public-config';

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
