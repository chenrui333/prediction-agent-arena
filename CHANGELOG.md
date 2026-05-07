# Changelog

All notable changes to `prediction-agent-arena` will be documented in this file.

This project follows semantic versioning for tagged releases.

## [0.1.0] - 2026-05-07

Initial local arena release for simulated prediction-market agent competitions.

### Backend And Arena Core

- Added Go backend using `net/http`, `chi`, `database/sql`, `modernc.org/sqlite`, Redis, and `slog`.
- Added SQLite migrations, WAL/foreign-key/busy-timeout setup, and short write transactions for order/fill/accounting paths.
- Added teams, registered agents, explicit round enrollment, market allowlists, heartbeats, decisions, orders, fills, positions, portfolio snapshots, score snapshots, risk events, settlements, admin actions, worker heartbeats, and request audit rows.
- Added deterministic fake venue with stable demo market identifiers, seeded price paths, simulated market outcomes, market price ticks, immediate marketable fills, and later open-order fills.
- Added average-cost portfolio accounting, realized PnL on reductions/settlement, cash checks, exposure checks, open-order reserves, and settlement payouts.
- Added composite scoring with return, risk, calibration/Brier, execution, and cost components.
- Added Redis-backed leaderboard cache and route rate-limit counters while keeping SQLite as the source of truth.

### APIs And Operator Workflows

- Added structured JSON API errors for auth, validation, risk rejection, not found, state conflicts, rate limits, and venue failures.
- Added agent-token-first auth with `paa_agent_...` tokens, token hashing, paused/revoked agent states, and optional legacy team-token auth.
- Added operator/admin APIs for teams, agents, rounds, enrollment, lock enforcement, market outcomes, settlement, exports, compaction, backup, health, and leaderboard freeze.
- Added round-scoped team reset, explicit all-round reset command, SQLite backup command, snapshot compaction, audit compaction, and access-packet generation for newly created/rotated tokens.
- Added locked-agent submissions for replay/evaluation rounds, activation preflight, completed-round lock immutability, and active-round lock replacement confirmation.
- Added `arenactl` and `just` workflows for local operation, seed, export, backup, health, team/agent creation, token rotation, round lifecycle, settlement, and maintenance.

### Frontend

- Added Next.js App Router frontend with arena overview, active-round summary, agent launchpad, leaderboard, evaluation projector view, team detail pages, and operator console.
- Added polling leaderboard and public team summary pages with active-round detail redaction.
- Added operator readiness panel covering backend/Redis/worker health, enrollment, markets, locked submissions, and round state.
- Added compatibility redirects from `/student` to `/agent` and `/leaderboard/finals` to `/leaderboard/evaluation`.

### SDK And Examples

- Added dependency-light Python Arena SDK for Python 3.11+ with typed dataclasses, structured exceptions, helper utilities, and conservative retries for safe calls.
- Added SDK helpers for basis-point conversion, probability clamping, edge calculation, and market outcome price lookup.
- Added Go random and momentum example agents.
- Added Python random example agent plus optional OpenAI and Anthropic probability-estimation templates.
- Added token hygiene defaults for examples and no admin API methods in the student-facing SDK.

### Documentation

- Added README quickstart, architecture, API examples, scoring formula, risk policy, SQLite/Redis operations, and roadmap.
- Added game logic, agent contract, participant quickstart, operator runbook, and bootcamp guide.
- Added agent-skill guides for building basic agents, building LLM-assisted agents, debugging risk rejections, preparing for evaluation rounds, and writing evaluation reports.
- Added `AGENTS.md` repository instructions for future coding agents.

### Security

- Enforced simulated/paper-only safety boundaries: no real-money trading, wallets, private keys, OAuth, Kubernetes, or production exchange credentials.
- Redacted public team activity during active rounds and public market metadata from agent-facing endpoints.
- Added local/exposed environment validation, admin token checks, legacy team-token-auth guardrails, and trusted proxy opt-in behavior.
- Added hashed request audit values using `ARENA_AUDIT_SALT` instead of storing raw IPs/user agents by default.
- Added exposed-mode requirements for strong admin token, strong audit salt, fail-closed rate limiting, disabled legacy team-token auth, and non-wildcard CORS origins.

### Changed

- Generalized product terminology from bootcamp/student/instructor wording to participant/agent/operator/evaluation wording while keeping `BOOTCAMP.md` as the bootcamp-specific guide.
- Kept legacy `/student` and `/leaderboard/finals` frontend paths as redirects to `/agent` and `/leaderboard/evaluation`.
- Changed seeded demo market display titles/categories to generalized arena wording while preserving stable `external_id` values for reseeding compatibility.

### CI And Tooling

- Added pinned local toolchain configuration through `mise.toml` for Go `1.26.2`, Node `24.15.0`, Python `3.14.4`, and `just`.
- Replaced Makefile workflows with `just` recipes.
- Added GitHub Actions CI for Go backend tests, frontend lint/typecheck/build, Docker Compose validation, Go example agents, and Python SDK tests across Python 3.11, 3.12, 3.13, and 3.14.
- Added Docker Compose local deployment for backend, worker, frontend, and Redis, plus exposed-mode override.

### Notes

- The default venue is the local fake venue. The Polymarket paper adapter remains an explicit safe skeleton and does not provide real trading functionality.
- Seeded fake market `external_id` values remain stable for local reseeding compatibility even though visible titles and categories use generalized arena terminology.
- This release is intended as the first local pilot baseline, not a hosted multi-tenant service.
