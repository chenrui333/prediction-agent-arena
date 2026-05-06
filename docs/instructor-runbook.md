# Instructor Runbook

This runbook assumes a local 9-10 student cohort and simulated/paper trading only.

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

The default admin token is `dev-admin-token` for local-only demos. Change `ARENA_ADMIN_TOKEN` in `.env` for a real class, especially if you bind beyond localhost.

For a class-network, Tailscale, firewall-protected host, or reverse-proxy setup, use exposed mode with strong secrets:

```bash
ARENA_ADMIN_TOKEN=$(openssl rand -base64 32)
ARENA_AUDIT_SALT=$(openssl rand -base64 32)
ARENA_RATE_LIMIT_ENABLED=true
ARENA_RATE_LIMIT_FAIL_CLOSED=true
```

Set `ARENA_ENV=exposed`, set `ARENA_ALLOWED_ORIGINS` to the frontend origin, and start with `just docker-up-exposed`. Redis remains local-only in the exposed override. Do not expose the app directly to the public internet.

## Venue Mode

Use the local deterministic venue for bootcamp pilots:

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

This creates 10 demo teams, `practice-1`, sample fake markets with deterministic price paths, and one default registered agent per team. It prints newly created agent tokens once. Store those tokens in a private class note.

## Create Teams

```bash
just create-team team-11 "Team 11"
```

The command prints the new token once. Existing token hashes cannot be converted back to token values.

Create the submitted/default student agent for that team:

```bash
just create-agent team-11 default "Team 11 Default Agent"
```

Give students the `paa_agent_...` token. Team tokens are not the default student credential; they are legacy-compatible only when `ARENA_LEGACY_TEAM_TOKEN_AUTH=true`.

Students can verify their credential with:

```bash
curl -sS -H "Authorization: Bearer $ARENA_API_TOKEN" http://localhost:8080/api/v1/me
```

## Start a Practice Round

```bash
just create-round practice-2 "Practice Round 2"
just activate-round practice-2
```

Use the admin UI if you prefer button controls.

## Monitor the Competition

- Project the leaderboard at http://localhost:3000/leaderboard. It refreshes every 5 seconds and shows the last updated time.
- Use http://localhost:3000/admin for team heartbeat, registered agents, equity, trade count, risk rejection count, exposure, round status, and exports.
- Use `/teams/{teamSlug}` pages to inspect recent decisions, orders, fills, and risk events.

The admin page stores the admin token only in that browser's local storage. Use `Forget token` on shared machines after class.

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

The new `paa_agent_...` token is printed once. The old token stops working immediately. Use this when a student leaks a token or resubmits a new official agent.

## Lock Submitted Agents

For replay/final-style rounds, lock each team to its submitted agent:

```bash
just lock-agent 1 final-1 abc123 team-01:final
just list-round-agents final-1
```

The lock stores `commit_sha` and `docker_image` with the round. Replay-mode rounds reject heartbeats, decisions, orders, and cancels from agents that are not locked to the round. Practice rounds remain open to active registered agents.

## Settle a Round

First resolve each market from the admin UI or API. Then run:

```bash
just settle-round practice-1
```

YES pays `10000` bps on yes and `0` on no. NO pays `10000` bps on no and `0` on yes. Settlement is idempotent, so re-running the command does not double-pay positions.

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

## Run the Final Round

1. Back up SQLite.
2. Create a final round with `just create-round final-1 "Final Round"`.
3. Activate it at the announced start time.
4. Monitor admin and leaderboard views.
5. Pause or resume teams only for instructor-approved infrastructure issues.
6. Resolve markets and run `just settle-round final-1`.
7. Complete the round with `just complete-round final-1`.
8. Freeze/export with `just freeze-leaderboard final-1` and `just export-round final-1`.
