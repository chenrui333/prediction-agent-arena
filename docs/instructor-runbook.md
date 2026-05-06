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

The default admin token is `dev-admin-token`. Change `ARENA_ADMIN_TOKEN` in `.env` for a real class.

## Seed Demo State

```bash
just seed
```

This creates 10 demo teams, `practice-1`, and sample fake markets. It prints newly created team tokens once. Store those tokens in a private class note.

## Create Teams

```bash
just create-team team-11 "Team 11"
```

The command prints the new token once. Existing token hashes cannot be converted back to token values.

## Start a Practice Round

```bash
just create-round practice-2 "Practice Round 2"
just activate-round practice-2
```

Use the admin UI if you prefer button controls.

## Monitor the Competition

- Project the leaderboard at http://localhost:3000/leaderboard.
- Use http://localhost:3000/admin for team heartbeat, equity, trade count, risk rejection count, and exposure.
- Use `/teams/{teamSlug}` pages to inspect recent decisions, orders, fills, and risk events.

## Pause a Bad Agent

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

Reset a round back to draft and clear round activity:

```bash
just reset-round practice-2
```

Use resets sparingly during scored rounds; they remove orders, fills, decisions, heartbeats, risk events, portfolios, and scores for the target scope.

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

## Recover From Redis Failure

Redis is not authoritative. It stores leaderboard cache entries and short-lived rate-limit counters only.

If Redis fails:

1. Leave backend running if it is otherwise healthy.
2. Restart Redis with `docker compose restart redis`.
3. Watch backend logs with `just logs`.
4. The next leaderboard request recomputes from SQLite and refreshes cache.

Orders, fills, portfolios, and scores remain in SQLite.

## Back Up SQLite

Best online backup if host `sqlite3` is installed:

```bash
mkdir -p backups
sqlite3 data/arena.db ".backup 'backups/arena-$(date +%Y%m%d-%H%M%S).db'"
```

Fallback file copy:

```bash
docker compose stop backend worker
mkdir -p backups
cp data/arena.db backups/arena-$(date +%Y%m%d-%H%M%S).db
docker compose start backend worker
```

SQLite runs in WAL mode. Avoid copying only `arena.db` while the backend is actively writing unless you use the `.backup` command.

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
6. Complete the round with `just complete-round final-1`.
7. Freeze/export with `just freeze-leaderboard final-1` and `just export-round final-1`.
