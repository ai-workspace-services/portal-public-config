import assert from 'node:assert/strict';
import test from 'node:test';
import { createApp } from '../src/index.ts';

test('GET /healthz and /readyz return cloudflare-worker runtime', async () => {
  const app = createApp();

  const healthRes = await app.request('https://portal-public-config-edge.workers.dev/healthz');
  assert.equal(healthRes.status, 200);
  assert.deepEqual(await healthRes.json(), {
    status: 'ok',
    runtime: 'cloudflare-worker',
    service: 'portal-public-config',
  });

  const readyRes = await app.request('https://portal-public-config-edge.workers.dev/readyz');
  assert.equal(readyRes.status, 200);
  assert.deepEqual(await readyRes.json(), {
    status: 'ready',
    runtime: 'cloudflare-worker',
  });
});

test('GET /api/auth/homepage-video is retired and returns 404', async () => {
  const app = createApp();

  const res = await app.request('https://www.svc.plus/api/auth/homepage-video');
  assert.equal(res.status, 404);
});

test('GET /api/internal/sandbox/guest and /api/internal/public-overview enforce token when set', async () => {
  const app = createApp();
  const env = {
    INTERNAL_SERVICE_TOKEN: 'secret-token',
    PUBLIC_OVERVIEW_USERS_FALLBACK: '128',
  };

  const unauthSandbox = await app.request(
    'https://portal-public-config-edge.workers.dev/api/internal/sandbox/guest',
    undefined,
    env,
  );
  assert.equal(unauthSandbox.status, 401);

  const authSandbox = await app.request(
    'https://portal-public-config-edge.workers.dev/api/internal/sandbox/guest',
    { headers: { 'X-Internal-Service-Token': 'secret-token' } },
    env,
  );
  assert.equal(authSandbox.status, 410);
  assert.deepEqual(await authSandbox.json(), {
    error: 'sandbox_retired',
    message: 'sandbox demo account has been retired',
  });

  const authOverview = await app.request(
    'https://portal-public-config-edge.workers.dev/api/internal/public-overview',
    { headers: { Authorization: 'Bearer secret-token' } },
    env,
  );
  assert.equal(authOverview.status, 200);
  const overview = (await authOverview.json()) as {
    registeredUsers: number;
    updatedAt: string;
    source: string;
  };
  assert.equal(overview.registeredUsers, 128);
  assert.equal(overview.source, 'cloudflare-worker');
  assert.ok(!Number.isNaN(Date.parse(overview.updatedAt)));
});

test('Internal endpoints succeed without token when INTERNAL_SERVICE_TOKEN is unset', async () => {
  const app = createApp();

  const sandboxRes = await app.request(
    'https://portal-public-config-edge.workers.dev/api/internal/sandbox/guest',
  );
  assert.equal(sandboxRes.status, 410);

  const overviewRes = await app.request(
    'https://portal-public-config-edge.workers.dev/api/internal/public-overview',
  );
  assert.equal(overviewRes.status, 200);
  const overview = (await overviewRes.json()) as {
    registeredUsers: number;
    source: string;
  };
  assert.equal(overview.registeredUsers, 0);
  assert.equal(overview.source, 'cloudflare-worker');
});
