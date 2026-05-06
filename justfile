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

frontend-install:
    cd frontend && npm ci

test: backend-test examples-test frontend-install
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

docker-down:
    docker compose down

logs:
    docker compose logs -f backend worker frontend

export:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round}}" mise exec -- go run ./cmd/arenactl export-round

create-team slug name="":
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" mise exec -- go run ./cmd/arenactl create-team --slug "{{slug}}" --name "{{name}}"

create-round round_slug=round name="":
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" mise exec -- go run ./cmd/arenactl create-round --slug "{{round_slug}}" --name "{{name}}"

activate-round round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" mise exec -- go run ./cmd/arenactl activate-round

pause-round round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" mise exec -- go run ./cmd/arenactl pause-round

complete-round round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" mise exec -- go run ./cmd/arenactl complete-round

reset-team team_slug=team:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" TEAM="{{team_slug}}" mise exec -- go run ./cmd/arenactl reset-team

pause-team team_slug=team:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" TEAM="{{team_slug}}" mise exec -- go run ./cmd/arenactl pause-team

resume-team team_slug=team:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" TEAM="{{team_slug}}" mise exec -- go run ./cmd/arenactl resume-team

reset-round round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" mise exec -- go run ./cmd/arenactl reset-round

freeze-leaderboard round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" mise exec -- go run ./cmd/arenactl freeze-leaderboard

export-round round_slug=round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" ROUND="{{round_slug}}" mise exec -- go run ./cmd/arenactl export-round

print-active-round:
    cd backend && ARENA_ADMIN_TOKEN="{{admin_token}}" mise exec -- go run ./cmd/arenactl print-active-round

print-team-tokens:
    cd backend && mise exec -- go run ./cmd/arenactl print-team-tokens
