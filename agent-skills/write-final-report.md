# Write The Final Report

Use this outline for the bootcamp final strategy report.

## 1. Strategy Summary

- What markets did your agent target?
- What signal or heuristic did it use?
- Did it trade YES, NO, or both?
- How did it size positions?

## 2. Agent Architecture

- Data sources used.
- Main loop frequency.
- Risk controls implemented in your code.
- Whether an LLM was used and how its output was validated.

## 3. Forecasting Quality

- Compare estimated probabilities to resolved outcomes.
- Discuss Brier score and calibration.
- Show one example of a good forecast and one bad forecast.

## 4. Execution Quality

- Trade count.
- Filled vs open/canceled/rejected orders.
- Typical limit price logic.
- Any slippage or stale order issues.

## 5. Risk Management

- Max exposure.
- Cash usage.
- Drawdown.
- Risk rejection codes encountered and fixes made.

## 6. Results

- Final equity.
- Return.
- Composite score.
- Rank.
- Best and worst trade.

## 7. Lessons Learned

- What worked.
- What failed.
- What you would change for a replay/final round.
- How you would improve with more data.

## Rules

- Do not include API tokens or private keys.
- Do not describe real-money trading instructions.
- Keep claims grounded in arena exports and logs.
