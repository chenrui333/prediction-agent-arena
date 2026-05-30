# Repository Instructions

@RTK.md

## Project

`prediction-agent-arena` is a local simulated/paper-trading prediction-market agent arena for cohorts, competitions, and agent evaluations.

Hard safety boundaries:

- No real-money trading.
- No wallets or production exchange credentials.
- No real Polymarket private-key trading.
- No production Kalshi integration in v1.
- SQLite is the source of truth.
- Redis is cache and rate limiting only.
- Public team activity is summary/redacted by default during active rounds.
- Evaluation/replay rounds should use registered agent locks instead of ad hoc team-token access.
- Practice signup can be ad hoc, but timed contest signup should close before the official round starts and use locked registered agents.
- Do not hardcode private Discord invites, practice signup links, contest signup links, admin tokens, or agent tokens in tracked files.

## Stack

- Backend: Go, `net/http`, `chi`, `database/sql`, `modernc.org/sqlite`, Redis, `slog`.
- Frontend: Next.js App Router, TypeScript, server components by default.
- Local orchestration: Docker Compose.

## Checks

Use these before committing meaningful changes:

```bash
rtk proxy mise exec -- just test
rtk proxy npm run build --prefix frontend
rtk proxy docker compose config --quiet
```

Backend-only:

```bash
rtk proxy mise exec -- just backend-test
rtk proxy mise exec -- gofmt -w <go-files>
```

`gofmt` comes from the pinned Go toolchain; use `mise exec -- gofmt` instead of installing a separate `gofmt` formula.

Frontend-only:

```bash
rtk proxy npm install --prefix frontend
rtk proxy npm run typecheck --prefix frontend
rtk proxy npm run lint --prefix frontend
rtk proxy npm run build --prefix frontend
```

Frontend conventions:

- Keep API helpers in `frontend/lib/api.ts`.
- Keep shared API types in `frontend/lib/types.ts`.
- Use server components by default; client components are for polling, local browser storage, and admin actions.
- Do not log or render admin tokens.

## Working Tree

Do not commit runtime artifacts:

- `data/`
- `logs/`
- `exports/`
- `frontend/node_modules/`
- `frontend/.next/`
