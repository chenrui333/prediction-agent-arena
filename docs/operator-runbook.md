# Operator Runbook

This runbook assumes a local 9-10 participant cohort and simulated/paper trading only.

## Setup

```bash
cp .env.example .env
mise install
just docker-up
```

Open:

- Frontend: http://localhost:3000
- Admin UI: http://localhost:3000/admin
- Health: http://localhost:8080/health

The default admin token is `dev-admin-token` for local-only demos. Change `ARENA_ADMIN_TOKEN` in `.env` for a real cohort, especially if you bind beyond localhost.

For a cohort network, Tailscale, firewall-protected host, or reverse-proxy setup, use exposed mode with strong secrets:

```bash
ARENA_ADMIN_TOKEN=$(openssl rand -base64 32)
ARENA_AUDIT_SALT=$(openssl rand -base64 32)
ARENA_RATE_LIMIT_ENABLED=true
ARENA_RATE_LIMIT_FAIL_CLOSED=true
```

Set `ARENA_ENV=exposed`, set `ARENA_ALLOWED_ORIGINS` to the frontend origin, keep `ARENA_LEGACY_TEAM_TOKEN_AUTH=false`, and start with `just docker-up-exposed`. Redis remains local-only in the exposed override. Do not expose the app directly to the public internet. Proxy headers are ignored unless `ARENA_TRUST_PROXY_HEADERS=true`; only enable that with a tight `ARENA_TRUSTED_PROXY_CIDRS` allowlist for your reverse proxy.

For always-on battle testing, Fly.io-style hosting, or a Vercel-hosted frontend, see [deployment-hosting.md](deployment-hosting.md). For practice signup, timed contest signup, token hygiene, Discord templates, and the same-app Fly preflight gate, see [onboarding.md](onboarding.md).

## Practice And Contest Signup

Use one public-safe onboarding hub and two private signup windows:

- Practice signup stays open in Discord for ad-hoc setup and synthetic-data agent iteration.
- Contest signup is announced in Discord with an open time, close time, and agent lock deadline.
- Practice leaderboard results are informal and may be reset.
- Official contest results come from a separate timed round after it is completed, frozen, and exported.

Keep practice and contest signup links out of tracked files and out of the public app. Pin them in Discord or send them privately.

## Venue Mode

Use the local deterministic venue for cohort pilots:

```env
ARENA_VENUE=fake
```

Fake markets are stored in SQLite, demo markets include deterministic price paths, and the worker advances prices during the round. Resolved market outcomes feed Brier/calibration scoring.

Open limit orders can fill later when the simulated price path crosses their limit. Portfolio accounting uses average cost: positions are simulated contract-cents, average entry prices are bps, and realized PnL is accumulated when positions are reduced or settled.

`ARENA_VENUE=polymarket_paper` is available only as an explicit optional skeleton. It validates `POLYMARKET_PAPER_BIN` and `POLYMARKET_PAPER_DATA_DIR`, but it does not enable wallet/private-key or real-money trading.

## Seed Demo State

```bash
just seed
```

This creates 10 demo teams, `practice-1`, sample fake markets with deterministic price paths, enrolls those teams in the round, and creates one default registered agent per team. It prints newly created agent tokens once and writes access packets under `exports/access/`. Store those tokens or packets in a private operator note.

## Create Teams

```bash
just create-team team-11 "Team 11"
```

The command prints the new token once. Existing token hashes cannot be converted back to token values.

Create the submitted/default agent for that team:

```bash
just create-agent team-11 default "Team 11 Default Agent"
```

Use `just create-agent-access team-11 default "Team 11 Default Agent"` when you want a one-time packet written to `exports/access/`. Give participants the `paa_agent_...` token. Team tokens are not the default participant credential; they are legacy-compatible only when `ARENA_LEGACY_TEAM_TOKEN_AUTH=true`.

Participants can verify their credential with:

```bash
curl -sS -H "Authorization: Bearer $ARENA_API_TOKEN" http://localhost:8080/api/v1/me
```

## Start a Practice Round

```bash
just create-round practice-2 "Practice Round 2"
just enroll-round-team team-11 practice-2
just activate-round practice-2
```

Enroll every participating team before activation. Use `just list-round-teams practice-2` to audit enrollment. Use the admin UI if you prefer button controls.

## Monitor the Competition

- Project the leaderboard at http://localhost:3000/leaderboard. It refreshes every 5 seconds and shows the last updated time.
- Use http://localhost:3000/leaderboard/evaluation for the larger projector view during evaluation presentations.
- Send participants to http://localhost:3000/agent to verify their agent token and copy local SDK/curl commands.
- Use http://localhost:3000/admin for readiness checks, team heartbeat, registered agents, equity, trade count, risk rejection count, exposure, round status, and exports.
- Use `/teams/{teamSlug}` pages for public summary views. During active competition rounds they hide decision reasons, orders, fills, and risk events to avoid strategy leakage.
- Use the admin UI or `GET /api/v1/admin/rounds/{round_id}/teams/{team_id}/activity` when you need full team activity.
- `ARENA_PUBLIC_TEAM_ACTIVITY=full` reveals full public detail only after round completion; active rounds remain summary/redacted.

The admin page stores the admin token only for the current browser tab/session. Use `Forget token` on shared machines after a session.

The agent launchpad uses in-memory state and optional browser-tab session storage rather than `localStorage`.

Mutation endpoints write hashed request audit rows to SQLite. Raw IP addresses and raw user agents are not stored; hashes use `ARENA_AUDIT_SALT`.

## Pause a Bad Agent

Pause just the submitted agent:

```bash
just pause-agent 1
```

Resume it with:

```bash
just resume-agent 1
```

Revoke a compromised agent token:

```bash
just revoke-agent 1
```

Pause the whole team and all its agents:

```bash
just pause-team team-03
```

Resume it with:

```bash
just resume-team team-03
```

Pause a full round with:

```bash
just pause-round practice-2
```

## Reset State

Reset a single team in the active/latest round:

```bash
just reset-team team-03
```

Reset a single team in a specific round:

```bash
ROUND=practice-2 just reset-team team-03
```

All-round team reset is intentionally explicit:

```bash
just reset-team-all-rounds team-03
```

Reset a round back to draft and clear round activity:

```bash
just reset-round practice-2
```

Use resets sparingly during scored rounds; they remove settlements, orders, fills, decisions, heartbeats, risk events, portfolios, and scores for the target scope.

## Rotate a Team Token

```bash
just rotate-team-token team-03
```

The new token is printed once. The old token stops working immediately. Existing token hashes cannot be printed.

## Rotate an Agent Token

```bash
just rotate-agent-token 1
```

The new `paa_agent_...` token is printed once. The old token stops working immediately. Use this when a participant leaks a token or resubmits a new official agent.

## Lock Submitted Agents

For replay/evaluation-style rounds, lock each team to its submitted agent:

```bash
just lock-agent 1 eval-1 abc123 team-01:evaluation
just list-round-agents eval-1
```

The lock stores `commit_sha` and `docker_image` with the round. Replay-mode rounds reject heartbeats, decisions, orders, and cancels from agents that are not locked to the round. Practice rounds remain open to active registered agents unless you explicitly require locked agents:

```bash
just require-locked-agents eval-1
just allow-unlocked-agents eval-1
```

Activating any round requires at least one active enrolled team. Activating a replay/locked round preflights active enrolled teams. Activation fails with `round_agent_locks_incomplete` until each active enrolled team has one active locked agent. Changing locks during an active round requires explicit confirmation and is written to the admin audit log with old/new agent IDs:

```bash
just lock-agent 1 eval-1 abc123 team-01:evaluation replace_active_round_lock
```

Completed-round locks are immutable.

## Settle a Round

First resolve each market from the admin UI or API. Then run:

```bash
just settle-round practice-1 settle_active_round true
```

YES pays `10000` bps on yes and `0` on no. NO pays `10000` bps on no and `0` on yes. Settlement rejects unresolved round markets, requires `settle_active_round` confirmation while the round is active, and is idempotent, so re-running the command does not double-pay positions.

## Freeze and Export Results

Freeze leaderboard artifacts:

```bash
just freeze-leaderboard practice-1
```

Export round results:

```bash
just export-round practice-1
```

Exports are written under `exports/{round_slug}/`. Event logs remain under `logs/{round_slug}/`.

Exports include:

- `leaderboard.csv`
- `scores.jsonl`
- `per_market_pnl.csv`
- `decision_quality.csv`
- `trade_report.csv`
- `calibration_bins.csv`
- `teams/{team_slug}.json`

## Recover From Redis Failure

Redis is not authoritative. It stores leaderboard cache entries and short-lived rate-limit counters only.

If Redis fails:

1. Leave backend running if it is otherwise healthy.
2. Restart Redis with `docker compose restart redis`.
3. Watch backend logs with `just logs`.
4. The next leaderboard request recomputes from SQLite and refreshes cache.

Orders, fills, portfolios, scores, registered agents, and audit records remain in SQLite. Local mode uses `ARENA_RATE_LIMIT_FAIL_CLOSED=false`, so route rate limits fail open if Redis is down. Exposed mode requires `ARENA_RATE_LIMIT_FAIL_CLOSED=true`.

## Back Up SQLite

Preferred online backup:

```bash
just backup-sqlite
```

This uses SQLite `VACUUM INTO` through Go and writes under `backend/backups/` by default.

Fallback file copy if needed:

```bash
docker compose stop backend worker
mkdir -p backups
cp data/arena.db backups/arena-$(date +%Y%m%d-%H%M%S).db
docker compose start backend worker
```

SQLite runs in WAL mode. Avoid copying only `arena.db` while the backend is actively writing unless you use the `.backup` command.

## Compact Snapshots

For long cohorts, reduce snapshot noise before archiving:

```bash
just compact-snapshots practice-1
```

The command keeps the latest snapshot for each team plus representative snapshots at the configured interval.

## Compact Audit Rows

Mutation request audit rows are retained in SQLite. Compact old raw audit rows after archiving:

```bash
just compact-audit 14d
```

This deletes `api_requests` rows older than the duration and records the compaction as an admin action.

## Health Checks

```bash
just health
curl -sS http://localhost:8080/health
```

Health reports DB status, Redis status, active round, latest market tick, latest portfolio snapshot, and worker freshness. Redis degradation should not corrupt arena state because SQLite is authoritative.

## Reset Demo State

For a clean local reset:

```bash
just docker-down
rm -rf data logs exports
just docker-up
just seed
```

This destroys local arena data.

## Run the Evaluation Round

1. Back up SQLite.
2. Create an evaluation round with `just create-round eval-1 "Evaluation Round"`.
3. Enroll participating teams, lock submitted agents, and run `just require-locked-agents eval-1`.
4. Check the admin readiness panel for DB/Redis/worker health, team enrollment, market catalog, and locked-agent counts.
5. Activate it at the announced start time.
6. Monitor admin, leaderboard, and evaluation projector views.
7. Pause or resume teams only for operator-approved infrastructure issues.
8. Resolve markets and run `just settle-round eval-1 settle_active_round true`.
9. Complete the round with `just complete-round eval-1` if you did not pass `complete_after_settlement=true`.
10. Freeze/export with `just freeze-leaderboard eval-1` and `just export-round eval-1`.
