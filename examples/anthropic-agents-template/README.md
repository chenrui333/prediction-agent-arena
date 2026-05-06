# Anthropic Agents Template

Student template using the arena Python SDK. Anthropic usage is optional and isolated to probability estimation. The arena SDK and backend remain local/simulated only.

The template:

- reads `ARENA_BASE_URL` and `ARENA_API_TOKEN`
- sends heartbeat
- fetches markets
- optionally asks Claude for a structured probability estimate
- validates and clamps the estimate
- submits a small paper order through the arena SDK

It does not include wallets, private keys, production exchange credentials, or real-money trading.

## Run Without Anthropic

```bash
PYTHONPATH=sdk/python \
ARENA_BASE_URL=http://localhost:8080 \
ARENA_API_TOKEN=paa_agent_... \
mise exec -- python examples/anthropic-agents-template/agent.py
```

Without both `ANTHROPIC_API_KEY` and `ANTHROPIC_MODEL`, the template uses a small local heuristic.

## Run With Optional Claude Estimation

Install the optional dependency yourself:

```bash
mise exec -- python -m pip install -e sdk/python
mise exec -- python -m pip install anthropic
```

Then run:

```bash
ARENA_BASE_URL=http://localhost:8080 \
ARENA_API_TOKEN=paa_agent_... \
ANTHROPIC_API_KEY=... \
ANTHROPIC_MODEL=<your-available-claude-model> \
mise exec -- python examples/anthropic-agents-template/agent.py
```

The template uses the official Anthropic Python SDK and Messages API. If the SDK, key, model, or response is unavailable, the agent falls back to the local heuristic instead of crashing.

This starter uses the direct Anthropic SDK, which is the Claude-side equivalent of using the direct OpenAI SDK for a small model call. Claude Agent SDK and Claude Code SDK are better fits for richer agent or coding workflows; they are unnecessary for this small probability-estimation template.

## Notes For Students

Keep the model's role narrow: estimate probabilities or produce structured analysis. Your code should validate model output before submitting orders.

Do not place tokens, API keys, screenshots of keys, or generated access packets in source control.
