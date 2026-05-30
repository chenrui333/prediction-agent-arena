# Deployment Hosting Guide

This guide compares the practical hosting options for always-on agent battle testing.
The recommended first hosted deployment runs the whole arena on a persistent
container host such as Fly.io. Vercel remains useful as an optional frontend-only
host, but it is not the simplest primary deployment target for this repo.

## Recommendation

Use a Fly.io-style full-stack deployment for v1:

- Run the Go API, snapshot worker, Next.js frontend, SQLite database, Redis, logs,
  and exports on persistent container infrastructure.
- Treat SQLite as the source of truth and Redis as cache/rate limiting only.
- Keep the public frontend and API on stable HTTPS origins.
- Use Vercel only when frontend preview deployments are worth the extra split-host
  configuration.

This preserves the existing operator workflow and avoids a backend redesign.

## Option Comparison

| Option | Fit | Tradeoffs |
| --- | --- | --- |
| Fly.io full-stack | Recommended hosted v1 path. Matches the current Docker, persistent volume, long-running API, and worker shape. | Requires Fly app/volume/process setup and a Redis choice. |
| Self-hosted Docker Compose | Best operational baseline. Runs the current backend, worker, Redis, SQLite volume, logs, and exports without code changes. | Operator owns host patching, HTTPS, backups, uptime, and deployment rollouts. |
| Hybrid Vercel frontend | Optional UI-only path. Vercel serves the Next.js app while the current backend stays on a persistent host. | Requires a public HTTPS backend URL and explicit CORS/env wiring across two hosts. |
| Vercel-only backend | Not recommended for v1. The current backend is a long-running Go service with a continuous worker and persistent local files. | Requires a backend redesign: managed SQL, external artifact storage, scheduled jobs, and function-compatible request handling. |

## Fly.io Full-Stack Deployment

Fly.io is the easiest hosted target for the current architecture because it can run
long-lived containers and attach persistent volumes for SQLite, logs, and exports.
Use one backend app for the API plus worker, one frontend app for Next.js, and
either a private Redis app or a managed Redis provider.

Recommended shape:

- Backend app: build from `Dockerfile.backend` with `CMD=arena-host`; this runs
  the API and snapshot worker in one process so both use the same SQLite volume.
- Frontend app: build from `Dockerfile.frontend`.
- SQLite/log/export volume: mount durable storage at paths used by
  `ARENA_DB_PATH`, `ARENA_LOG_DIR`, and `ARENA_EXPORT_DIR`.
- Redis: keep private to the app/network; it is not authoritative.

Example Fly config templates live at:

- [backend.fly.toml.example](../deploy/fly/backend.fly.toml.example)
- [frontend.fly.toml.example](../deploy/fly/frontend.fly.toml.example)

Set `ARENA_ADMIN_TOKEN` and `ARENA_AUDIT_SALT` as Fly secrets, not checked-in
config.

Initial Fly setup:

```bash
cp deploy/fly/backend.fly.toml.example deploy/fly/backend.local.toml
cp deploy/fly/frontend.fly.toml.example deploy/fly/frontend.local.toml
```

Edit the copied `*.local.toml` files for the chosen app names, region, frontend
host, API host, and Redis address. Then create the backend app and volume:

```bash
flyctl apps create <backend-app>
flyctl volumes create arena_data --app <backend-app> --region <region> --size 3
flyctl secrets set --app <backend-app> \
  ARENA_ADMIN_TOKEN=<32-plus-character-secret> \
  ARENA_AUDIT_SALT=<16-plus-character-secret>
flyctl deploy --config deploy/fly/backend.local.toml --remote-only
```

Create and deploy the frontend app after the backend API hostname is known:

```bash
flyctl apps create <frontend-app>
flyctl deploy --config deploy/fly/frontend.local.toml --remote-only
```

Keep the backend app at one machine while SQLite is the source of truth. Fly
Volumes are local to one machine and are not shared storage. Move to LiteFS or a
managed SQL database before scaling the backend horizontally.

Required runtime environment:

```env
ARENA_ENV=exposed
ARENA_ADMIN_TOKEN=<32-plus-character-secret>
ARENA_AUDIT_SALT=<16-plus-character-secret>
ARENA_ALLOWED_ORIGINS=https://<frontend-host>
ARENA_FRONTEND_ORIGIN=https://<frontend-host>
ARENA_PUBLIC_TEAM_ACTIVITY=summary
ARENA_LEGACY_TEAM_TOKEN_AUTH=false
ARENA_RATE_LIMIT_ENABLED=true
ARENA_RATE_LIMIT_FAIL_CLOSED=true
ARENA_DB_PATH=/data/arena.db
ARENA_LOG_DIR=/data/logs
ARENA_EXPORT_DIR=/data/exports
ARENA_REDIS_ADDR=<private-redis-host>:6379
NEXT_PUBLIC_ARENA_API_BASE_URL=https://<backend-api-host>
```

Use separate public hostnames for frontend and API unless a reverse proxy is added
to serve both from one origin. If the frontend and API use different origins, keep
`ARENA_ALLOWED_ORIGINS` restricted to the frontend URL and set
`NEXT_PUBLIC_ARENA_API_BASE_URL` to the API URL at frontend build time.

### Fly Pilot Gate

For the current same-app Fly pilot, run the low-load gate before practice or
contest signup invites:

~~~bash
scripts/fly_pilot_gate.sh
~~~

The gate reads .env.fly.local, uses or creates a reserved smoke-fly team, stores
the generated smoke agent token only under ignored access-packets/fly/, and does
not print tokens. It checks backend health, frontend root, frontend /onboard,
CORS, public APIs, admin reads, agent auth, heartbeat, one small valid order,
one intentional risk rejection, and a small read-only probe.

For a stronger pre-invite check without meaningful load, run it twice five
minutes apart:

~~~bash
scripts/fly_pilot_gate.sh
sleep 300
scripts/fly_pilot_gate.sh
~~~

The cleanup step removes the smoke agent lock when locked-agent mode is enabled,
resets the smoke team in the active round, withdraws it, and pauses it. The
public leaderboard includes only active enrolled teams, so the reserved operator
row is removed from standings after cleanup.

For practice signup, timed contest signup, participant-facing copy, and Discord
templates, see [onboarding.md](onboarding.md). Keep live Discord invite links
and signup links out of tracked files. If you want the hosted /onboard page to
show a Discord join button, set NEXT_PUBLIC_DISCORD_INVITE_URL only in the
frontend hosting environment.

Before a scored session, verify:

```bash
curl -sS https://<backend-api-host>/health
curl -sS https://<backend-api-host>/api/v1/leaderboard
```

Then verify from the hosted UI:

- `/` loads market and active-round data.
- `/leaderboard` refreshes without browser CORS errors.
- `/agent` validates a `paa_agent_...` token without storing it in local storage.
- `/admin` accepts the admin token and can read health, teams, rounds, and agents.
- A sample agent can call `/api/v1/me`, heartbeat, and submit a simulated order.

Use registered agent tokens as the participant allowlist for hosted tests. Only
issue `paa_agent_...` tokens to teams that should run agents, rotate leaked tokens,
and keep `ARENA_LEGACY_TEAM_TOKEN_AUTH=false`. The admin token remains
operator-only and should be stored as a platform secret.

## Self-Hosted Deployment

Use full self-hosting when operator control matters more than managed platform
deployment. This remains the canonical deployment shape for classroom pilots and
private networks.

Use the existing exposed deployment path:

```bash
ARENA_ENV=exposed
ARENA_ADMIN_TOKEN=<32-plus-character-secret>
ARENA_AUDIT_SALT=<16-plus-character-secret>
ARENA_ALLOWED_ORIGINS=https://<frontend-host>
ARENA_FRONTEND_ORIGIN=https://<frontend-host>
ARENA_PUBLIC_TEAM_ACTIVITY=summary
ARENA_LEGACY_TEAM_TOKEN_AUTH=false
ARENA_RATE_LIMIT_ENABLED=true
ARENA_RATE_LIMIT_FAIL_CLOSED=true
just docker-up-exposed
```

For self-hosted frontend and backend on one origin, set:

```env
NEXT_PUBLIC_ARENA_API_BASE_URL=https://<backend-api-host>
ARENA_FRONTEND_ORIGIN=https://<frontend-host>
ARENA_ALLOWED_ORIGINS=https://<frontend-host>
```

Back up `data/arena.db` with the built-in SQLite backup workflow and copy
`exports/` after each scored round. `logs/` are useful for postmortems and should
be retained at least through grading.

## Hybrid Vercel Frontend

Use this only when Vercel preview deployments or Vercel-managed frontend hosting
are worth the extra split-host setup. The arena core still needs a persistent
backend host.

### Backend host

Run the current Docker Compose stack on a persistent host with durable directories
for `data/`, `logs/`, and `exports/`. A small VPS, a VM, Fly.io, or another
container host with attached persistent disk is enough for initial always-on
battle testing.

Required backend environment:

```env
ARENA_ENV=exposed
ARENA_ADMIN_TOKEN=<32-plus-character-secret>
ARENA_AUDIT_SALT=<16-plus-character-secret>
ARENA_ALLOWED_ORIGINS=https://<vercel-app-host>
ARENA_FRONTEND_ORIGIN=https://<vercel-app-host>
ARENA_PUBLIC_TEAM_ACTIVITY=summary
ARENA_LEGACY_TEAM_TOKEN_AUTH=false
ARENA_RATE_LIMIT_ENABLED=true
ARENA_RATE_LIMIT_FAIL_CLOSED=true
```

Start the exposed stack:

```bash
just docker-up-exposed
```

Put the backend behind HTTPS before sharing it with participants; do not expose
the container port directly. Keep Redis bound to localhost or a private network.
Only enable `ARENA_TRUST_PROXY_HEADERS` when the backend is behind a trusted
reverse proxy and `ARENA_TRUSTED_PROXY_CIDRS` matches that proxy.

### Vercel frontend

Create a Vercel project for this repository with:

- Root Directory: `frontend`
- Framework Preset: Next.js
- Build Command: Vercel default for Next.js, or `npm run build`
- Environment variable:

```env
NEXT_PUBLIC_ARENA_API_BASE_URL=https://<backend-api-host>
```

After the first Vercel deployment, update the backend values:

```env
ARENA_ALLOWED_ORIGINS=https://<vercel-app-host>
ARENA_FRONTEND_ORIGIN=https://<vercel-app-host>
```

Redeploy or restart the backend after changing those values.

## Vercel-Only Future Work

A Vercel-only backend should be treated as a separate migration, not a deployment
configuration change. The minimum design work is:

- Replace SQLite file persistence with a managed SQL database and update queries,
  migrations, backup, and transaction assumptions.
- Replace local `logs/` and `exports/` writes with durable object/blob storage.
- Replace the continuous snapshot worker with scheduled invocations or a
  provider-native background worker.
- Keep Redis-compatible rate limiting and leaderboard caching through a managed
  Redis provider.
- Re-check Vercel function runtime limits before implementation, especially
  request duration, payload size, background execution, and Go runtime support.

Do this only after the Fly.io or self-hosted deployment proves the hosted usage
pattern and the expected concurrency/retention requirements are clear.
