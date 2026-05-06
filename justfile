set shell := ["bash", "-euo", "pipefail", "-c"]

admin_token := env_var_or_default("ARENA_ADMIN_TOKEN", "dev-admin-token")
round := env_var_or_default("ROUND", "practice-1")
team := env_var_or_default("TEAM", "")

default:
    just --list

dev:
    @echo "Run 'just docker-up', then 'just seed' in another terminal."

backend-test:
    cd backend && mise exec -- go test ./...

examples-test:
    cd examples/random-agent && mise exec -- go test ./...
    cd examples/momentum-agent && mise exec -- go test ./...

python-sdk-test:
    PYTHONPATH=sdk/python mise exec -- python -m unittest discover -s sdk/python/tests

python-examples-test:
    PYTHONPATH=sdk/python mise exec -- python -m py_compile examples/python-random-agent/agent.py examples/openai-agents-template/agent.py examples/anthropic-agents-template/agent.py

frontend-install:
    cd frontend && npm ci

test: backend-test examples-test python-sdk-test python-examples-test frontend-install
    cd frontend && npm run typecheck
    cd frontend && npm run lint

lint:
    cd frontend && npm run lint

fmt:
    mise exec -- gofmt -w backend examples/random-agent examples/momentum-agent

seed:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" mise exec -- go run ./cmd/arenactl seed-demo

docker-up:
    docker compose up --build

docker-up-exposed:
    docker compose -f docker-compose.yml -f docker-compose.exposed.yml up --build

docker-down:
    docker compose down

logs:
    docker compose logs -f backend worker frontend

export:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round}}" mise exec -- go run ./cmd/arenactl export-round

create-team slug name="":
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" mise exec -- go run ./cmd/arenactl create-team --slug "{{slug}}" --name "{{name}}"

create-agent team_slug=team agent_slug="default" name="":
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" TEAM="{{team_slug}}" mise exec -- go run ./cmd/arenactl create-agent --slug "{{agent_slug}}" --name "{{name}}"

create-agent-access team_slug=team agent_slug="default" name="":
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" TEAM="{{team_slug}}" mise exec -- go run ./cmd/arenactl create-agent --slug "{{agent_slug}}" --name "{{name}}" --write-access-packet

create-round round_slug=round name="":
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" mise exec -- go run ./cmd/arenactl create-round --slug "{{round_slug}}" --name "{{name}}"

activate-round round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" mise exec -- go run ./cmd/arenactl activate-round

pause-round round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" mise exec -- go run ./cmd/arenactl pause-round

complete-round round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" mise exec -- go run ./cmd/arenactl complete-round

require-locked-agents round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" mise exec -- go run ./cmd/arenactl require-locked-agents

allow-unlocked-agents round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" mise exec -- go run ./cmd/arenactl allow-unlocked-agents

settle-round round_slug=round confirm="" complete_after_settlement="false":
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" mise exec -- go run ./cmd/arenactl settle-round --confirm "{{confirm}}" --complete-after-settlement="{{complete_after_settlement}}"

enroll-round-team team_slug=team round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" TEAM="{{team_slug}}" mise exec -- go run ./cmd/arenactl enroll-round-team

pause-round-team team_slug=team round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" TEAM="{{team_slug}}" mise exec -- go run ./cmd/arenactl pause-round-team

resume-round-team team_slug=team round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" TEAM="{{team_slug}}" mise exec -- go run ./cmd/arenactl resume-round-team

withdraw-round-team team_slug=team round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" TEAM="{{team_slug}}" mise exec -- go run ./cmd/arenactl withdraw-round-team

list-round-teams round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" mise exec -- go run ./cmd/arenactl list-round-teams

reset-team team_slug=team:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round}}" TEAM="{{team_slug}}" mise exec -- go run ./cmd/arenactl reset-team

reset-team-all-rounds team_slug=team:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" TEAM="{{team_slug}}" mise exec -- go run ./cmd/arenactl reset-team-all-rounds --confirm all_rounds

rotate-team-token team_slug=team:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" TEAM="{{team_slug}}" mise exec -- go run ./cmd/arenactl rotate-team-token

rotate-agent-token agent_id:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" mise exec -- go run ./cmd/arenactl rotate-agent-token --agent-id "{{agent_id}}"

rotate-agent-token-access agent_id:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" mise exec -- go run ./cmd/arenactl rotate-agent-token --agent-id "{{agent_id}}" --write-access-packet

pause-team team_slug=team:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" TEAM="{{team_slug}}" mise exec -- go run ./cmd/arenactl pause-team

resume-team team_slug=team:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" TEAM="{{team_slug}}" mise exec -- go run ./cmd/arenactl resume-team

pause-agent agent_id:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" mise exec -- go run ./cmd/arenactl pause-agent --agent-id "{{agent_id}}"

resume-agent agent_id:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" mise exec -- go run ./cmd/arenactl resume-agent --agent-id "{{agent_id}}"

revoke-agent agent_id:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" mise exec -- go run ./cmd/arenactl revoke-agent --agent-id "{{agent_id}}"

lock-agent agent_id round_slug=round commit_sha="" docker_image="" confirm="":
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" mise exec -- go run ./cmd/arenactl lock-agent --agent-id "{{agent_id}}" --commit-sha "{{commit_sha}}" --docker-image "{{docker_image}}" --confirm "{{confirm}}"

list-round-agents round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" mise exec -- go run ./cmd/arenactl list-round-agents

reset-round round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" mise exec -- go run ./cmd/arenactl reset-round

freeze-leaderboard round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" mise exec -- go run ./cmd/arenactl freeze-leaderboard

export-round round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" mise exec -- go run ./cmd/arenactl export-round

compact-snapshots round_slug=round keep_every="5m":
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" mise exec -- go run ./cmd/arenactl compact-snapshots --keep-every "{{keep_every}}"

compact-audit older_than="14d":
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" mise exec -- go run ./cmd/arenactl compact-audit --older-than "{{older_than}}"

backup-sqlite:
    cd backend && mise exec -- go run ./cmd/arenactl backup-sqlite

health:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" mise exec -- go run ./cmd/arenactl health

print-active-round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" mise exec -- go run ./cmd/arenactl print-active-round

print-team-tokens:
    cd backend && mise exec -- go run ./cmd/arenactl print-team-tokens
