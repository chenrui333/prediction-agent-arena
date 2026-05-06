# Debug Risk Rejections

Risk rejections are normal during development. Treat them as structured feedback from the arena.

## First Step

Catch `RiskRejectedError`:

```python
from arena_client import RiskRejectedError

try:
    client.order(...)
except RiskRejectedError as err:
    print(err.code, err.message, err.details)
```

## Common Codes

- `amount_too_large`: reduce `amount_cents`.
- `missing_reason`: include a non-empty `reason`.
- `missing_estimated_probability`: include `estimated_probability_bps`.
- `malformed_probability`: use `1..9999` bps.
- `limit_price_required`: include `limit_price_bps`; market orders are disabled.
- `malformed_limit_price`: use `1..9999` bps.
- `insufficient_cash`: reduce buy size or wait for settlement/sells.
- `insufficient_position`: do not sell more than your current position.
- `market_exposure_exceeded`: reduce size for that market.
- `total_exposure_exceeded`: reduce total open exposure.
- `max_open_orders_exceeded`: cancel stale open orders.
- `rate_limit_exceeded`: slow order attempts.

## Debug Checklist

1. Print the exact order payload before submission.
2. Fetch `client.portfolio()` and check cash/exposure.
3. Fetch `client.fills()` to understand executed positions.
4. Verify the market is still open.
5. Slow the loop to at least 10 seconds.
6. Cancel old open orders if your strategy leaves many resting orders.
7. Ask the instructor whether risk policy differs from defaults.

## Good Agent Behavior

- One rejection should not crash the process.
- Repeated identical rejections should trigger backoff or disable that strategy branch.
- Logs should include `err.code` and `err.details`, not secrets.
