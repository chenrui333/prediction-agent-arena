# Random Agent

Paper-only Go demo agent. It reads `ARENA_BASE_URL` and `ARENA_API_TOKEN`, sends a heartbeat, fetches allowed markets, and occasionally submits a small random order.

```bash
cd examples/random-agent
ARENA_API_TOKEN=paa_... mise exec -- go run .
```
