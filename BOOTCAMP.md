# Bootcamp Guide

## Structure

Two-week format for 9-10 students:

1. Days 1-2: API orientation, fake market mechanics, safety rules, baseline agents.
2. Days 3-5: Agent design, risk handling, portfolio observation, decision logging.
3. Days 6-8: Strategy iteration, calibration, execution quality, leaderboard reviews.
4. Days 9-10: Final paper round, exports, demos, and postmortems.

## Student Deliverables

- A runnable agent with setup instructions.
- Heartbeat support.
- Market observation loop.
- Decision and order submission.
- Risk-aware sizing.
- A short strategy note covering signal inputs, calibration, and failure modes.
- Final demo using only simulated/paper-trading arena APIs.

## Recommended Agent Architecture

- `config`: reads `ARENA_BASE_URL`, `ARENA_API_TOKEN`, and local strategy settings.
- `client`: small HTTP wrapper with retries and clear error logging.
- `observer`: fetches markets, fills, and portfolio state.
- `strategy`: estimates probabilities and edge.
- `risk`: caps order size and avoids repeated rejected orders.
- `executor`: submits decisions/orders and records local traces.

## Grading Rubric

- 30% strategy clarity and calibration discipline.
- 25% engineering reliability and clean API usage.
- 20% risk management and rejection avoidance.
- 15% execution quality and observability.
- 10% final presentation and postmortem.

Leaderboard rank can inform grading, but it should not be the whole grade.

## Policies

- Simulated trading only.
- No real funds.
- No wallets or production exchange credentials.
- Students use registered agent tokens, not shared team credentials, unless the instructor explicitly enables legacy team-token auth for a local exercise.
- Teams must be explicitly enrolled in a round before their agents can trade in that round.
- For final/replay rounds, instructors should lock each team to its submitted registered agent, commit SHA, and Docker image before the round starts, then enable locked-agent enforcement for the round.
- Locked/replay round activation preflights active enrolled teams and should fail until each active enrolled team has one active locked agent.
- Public team pages should remain summary/redacted during active rounds to avoid leaking decision reasons, orders, fills, or risk events to competitors.
- Public market endpoints hide instructor metadata; do not place student-visible instructions only in `metadata_json`.
- No direct external mutation of arena state outside the documented API.
- No direct DB writes by student agents.
- External data policy is instructor-configurable. A typical default is:
  - Allowed: public read-only web data, local files, model outputs, and team-authored data.
  - Forbidden: private credentials, paid data sources without class approval, real-money trading APIs, and direct mutation of external markets.

## Instructor Checklist

- Start Docker Compose.
- Run `just seed`.
- Save printed agent tokens or generated access packets in a private class note.
- Assign one registered agent token per team.
- Lock final submitted agents with `just lock-agent` and `just require-locked-agents` before a scored final round.
- Confirm http://localhost:3000/leaderboard updates.
- Run one example agent before students begin.
- Export results after each scored round.

## Competition Cadence

- Practice rounds should be short and forgiving.
- Final rounds should be announced with start/end times.
- Pause a round when debugging instructor infrastructure.
- Reset a team only for setup mistakes or explicit instructor-approved retries.
- Use admin-only activity views for live troubleshooting; save full public postmortems until after the round is complete.
- Review rejected orders as teaching material; they are part of the execution score.
