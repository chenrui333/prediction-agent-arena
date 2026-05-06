# Build A Basic Agent

Use this checklist to build a reliable non-LLM arena agent.

## Goal

Create an observe-decide-act loop that submits low-frequency, risk-aware paper orders.

## Inputs

- `ARENA_BASE_URL`
- `ARENA_API_TOKEN`
- public markets
- authenticated portfolio
- recent fills

## Loop

1. Create `ArenaClient.from_env()`.
2. Call `client.me()` once at startup and print team/agent identity.
3. Send `client.heartbeat()` every loop.
4. Fetch `client.markets()`.
5. Filter to open markets.
6. Fetch `client.portfolio()` if your strategy depends on cash/exposure.
7. Estimate probability in bps.
8. Validate:
   - `1 <= estimated_probability_bps <= 9999`
   - `1 <= limit_price_bps <= 9999`
   - `amount_cents <= 50000` unless instructor changed policy
   - reason is non-empty
9. Submit `client.order(...)`.
10. Catch `RiskRejectedError` and back off.
11. Sleep at least 10-30 seconds before the next order attempt.

## Minimal Strategy Ideas

- Random baseline with small order size.
- Momentum: buy YES when price rises over recent ticks.
- Mean reversion: buy YES when price drops below a moving average.
- Calibration-only: submit decisions without many orders.

## Quality Bar

- No tight loops.
- No hardcoded tokens.
- No crashes on one bad response.
- Logs include structured error code and message.
- The agent can restart without manual state cleanup.
