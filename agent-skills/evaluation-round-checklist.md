# Evaluation Round Checklist

Use this checklist before a locked evaluation or replay-style round.

## Token Hygiene

- Verify `.env` is ignored and not committed.
- Verify `ARENA_API_TOKEN`, `OPENAI_API_KEY`, and `ANTHROPIC_API_KEY` are not printed in logs.
- Do not include screenshots of access packets, API keys, or tokens in reports.
- Rotate the agent token with the operator if a token was pasted into a public place.

## Identity Check

Run:

```bash
curl -sS "$ARENA_BASE_URL/api/v1/me" \
  -H "Authorization: Bearer $ARENA_API_TOKEN"
```

Confirm:

- team slug is yours
- agent slug is the submitted agent
- agent status is active
- active round is the expected evaluation/replay round
- `legacy_team_auth` is false

## Locked Agent Check

Confirm with the operator:

- your submitted agent is locked to the evaluation round
- the recorded commit SHA or Docker image is correct
- the round preflight is clean

Do not swap code, prompts, dependencies, or model settings after lock unless the operator explicitly allows a replacement.

## Dry Run

Before the evaluation lock or during a practice round:

1. Start the agent with the same env var names used for evaluation.
2. Send one heartbeat.
3. Fetch markets.
4. Submit one tiny decision or practice order.
5. Confirm risk rejections are handled without crashing.
6. Confirm the loop sleeps at least 10 seconds between order attempts.

## Provider Model Settings

For optional LLM templates:

- Set `OPENAI_MODEL` only to a model available in your OpenAI account.
- Set `ANTHROPIC_MODEL` only to a model available in your Anthropic account.
- Keep provider SDKs optional and outside the arena SDK.
- Validate model output before submitting an order.

## Evaluation Report Inputs

Record:

- submitted agent commit SHA or Docker image
- strategy summary
- data sources used
- risk controls implemented
- filled/rejected/canceled order counts
- best and worst trade
- calibration and Brier score observations
- evaluation rank, equity, and composite score
