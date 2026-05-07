# Changelog

All notable changes to `prediction-agent-arena` will be documented in this file.

This project follows semantic versioning for tagged releases.

## [0.1.0] - 2026-05-07

Initial local arena release for simulated prediction-market agent competitions.

### Added

- Go backend with `chi`, SQLite migrations, Redis-backed cache/rate limiting, structured errors, JSONL event logs, and graceful local operation.
- Deterministic fake venue with market allowlists, seeded price paths, open-order fills, portfolio snapshots, market outcomes, settlement, and Brier/calibration-aware scoring.
- Registered agent model with `paa_agent_...` tokens, token hashing, lifecycle controls, round-scoped locked submissions, team enrollment, request audit rows, and exposed-mode guardrails.
- Operator APIs and `arenactl`/`just` workflows for teams, agents, rounds, enrollment, locks, settlement, exports, SQLite backup, snapshot/audit compaction, and local health checks.
- Next.js App Router frontend with arena overview, agent launchpad, leaderboard, evaluation projector view, team detail pages, and operator console.
- Python Arena SDK with typed dataclasses, structured exceptions, helper utilities, conservative retries, and example random/OpenAI/Anthropic agents.
- Documentation for game logic, agent contract, participant quickstart, operator runbook, bootcamp usage, agent skills, Docker Compose, risk policy, scoring, and operations.
- CI coverage for Go backend, frontend lint/typecheck/build, Go example agents, Python SDK across Python 3.11-3.14, and Docker Compose validation.

### Security

- Enforced simulated/paper-only safety boundaries: no real-money trading, wallets, private keys, OAuth, Kubernetes, or production exchange credentials.
- Redacted public team activity during active rounds and public market metadata from agent-facing endpoints.
- Added local/exposed environment validation, admin token checks, legacy team-token-auth guardrails, and trusted proxy opt-in behavior.

### Changed

- Generalized product terminology from bootcamp/student/instructor wording to participant/agent/operator/evaluation wording while keeping `BOOTCAMP.md` as the bootcamp-specific guide.
- Kept legacy `/student` and `/leaderboard/finals` frontend paths as redirects to `/agent` and `/leaderboard/evaluation`.

### Notes

- The default venue is the local fake venue. The Polymarket paper adapter remains an explicit safe skeleton and does not provide real trading functionality.
- Seeded fake market `external_id` values remain stable for local reseeding compatibility even though visible titles and categories use generalized arena terminology.
