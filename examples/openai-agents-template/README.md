# OpenAI Agents Template

Student template using the arena Python SDK. OpenAI usage is optional and isolated to probability estimation. The arena SDK and backend remain local/simulated only.

The template:

- reads `ARENA_BASE_URL` and `ARENA_API_TOKEN`
- sends heartbeat
- fetches markets
- optionally asks OpenAI for a structured probability estimate
- validates and clamps the estimate
- submits a small paper order through the arena SDK

It does not include wallets, private keys, production exchange credentials, or real-money trading.

## Run Without OpenAI

```bash
PYTHONPATH=sdk/python \
ARENA_BASE_URL=http://localhost:8080 \
ARENA_API_TOKEN=paa_agent_... \
mise exec -- python examples/openai-agents-template/agent.py
```

Without `OPENAI_API_KEY`, the template uses a small local heuristic.

## Run With Optional OpenAI Estimation

Install the optional dependency yourself:

```bash
mise exec -- python -m pip install -e sdk/python
mise exec -- python -m pip install openai
```

Then run:

```bash
ARENA_BASE_URL=http://localhost:8080 \
ARENA_API_TOKEN=paa_agent_... \
OPENAI_API_KEY=... \
OPENAI_MODEL=gpt-5.4-mini \
mise exec -- python examples/openai-agents-template/agent.py
```

The OpenAI call uses the Responses API with structured JSON output. If the SDK, key, model, or response is unavailable, the agent falls back to the local heuristic instead of crashing.

## Notes For Students

Keep the model's role narrow: estimate probabilities or produce structured analysis. Your code should validate model output before submitting orders.

Do not place tokens, API keys, screenshots of keys, or generated access packets in source control.
