# Python Random Agent

Beginner-friendly baseline Python agent using the student SDK. It has no LLM dependency and does not perform real-money trading.

## Run

From the repo root:

```bash
mise exec -- python -m pip install -e sdk/python
ARENA_BASE_URL=http://localhost:8080 \
ARENA_API_TOKEN=paa_agent_... \
mise exec -- python examples/python-random-agent/agent.py
```

Or without installing:

```bash
PYTHONPATH=sdk/python \
ARENA_BASE_URL=http://localhost:8080 \
ARENA_API_TOKEN=paa_agent_... \
mise exec -- python examples/python-random-agent/agent.py
```

The agent sends heartbeats, fetches public markets, and submits small low-frequency buy-only random orders. It avoids intentional sell-without-position rejections so a first run is quieter, but it still catches risk rejections and backs off naturally through its loop delay.
