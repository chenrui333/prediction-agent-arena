#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ENV_FILE="$(printenv ENV_FILE 2>/dev/null || true)"
ACCESS_DIR="$(printenv ACCESS_DIR 2>/dev/null || true)"
SMOKE_TEAM_SLUG="$(printenv SMOKE_TEAM_SLUG 2>/dev/null || true)"
SMOKE_TEAM_NAME="$(printenv SMOKE_TEAM_NAME 2>/dev/null || true)"
SMOKE_AGENT_SLUG="$(printenv SMOKE_AGENT_SLUG 2>/dev/null || true)"
SMOKE_AGENT_NAME="$(printenv SMOKE_AGENT_NAME 2>/dev/null || true)"
CONCURRENCY="$(printenv CONCURRENCY 2>/dev/null || true)"
REQUESTS="$(printenv REQUESTS 2>/dev/null || true)"

if [[ -z "$ENV_FILE" ]]; then ENV_FILE="$ROOT_DIR/.env.fly.local"; fi
if [[ -z "$ACCESS_DIR" ]]; then ACCESS_DIR="$ROOT_DIR/access-packets/fly"; fi
if [[ -z "$SMOKE_TEAM_SLUG" ]]; then SMOKE_TEAM_SLUG="smoke-fly"; fi
if [[ -z "$SMOKE_TEAM_NAME" ]]; then SMOKE_TEAM_NAME="Fly Smoke Test"; fi
if [[ -z "$SMOKE_AGENT_SLUG" ]]; then SMOKE_AGENT_SLUG="default"; fi
if [[ -z "$SMOKE_AGENT_NAME" ]]; then SMOKE_AGENT_NAME="Fly Smoke Agent"; fi
if [[ -z "$CONCURRENCY" ]]; then CONCURRENCY="3"; fi
if [[ -z "$REQUESTS" ]]; then REQUESTS="45"; fi

tmp_dir=""
round_id=""
team_id=""
cleanup_ready=0

die() {
  echo "FAIL $*" >&2
  exit 1
}

pass() {
  echo "PASS $*"
}

load_env_file() {
  [[ -f "$ENV_FILE" ]] || die "missing env file: $ENV_FILE"

  local line key value
  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ "$line" =~ ^[[:space:]]*$ ]] && continue
    [[ "$line" =~ ^[[:space:]]*# ]] && continue
    [[ "$line" == *"="* ]] || continue

    key="$(printf '%s' "$line" | sed 's/[[:space:]]//g; s/=.*//')"
    value="$(printf '%s' "$line" | sed 's/^[^=]*=//; s/^[[:space:]]*//; s/[[:space:]]*$//; s/^"\(.*\)"$/\1/')"

    case "$key" in
      ARENA_ADMIN_TOKEN|ARENA_FRONTEND_ORIGIN|NEXT_PUBLIC_ARENA_API_BASE_URL)
        printf -v "$key" '%s' "$value"
        export "$key"
        ;;
    esac
  done <"$ENV_FILE"
}

require_tools() {
  command -v curl >/dev/null 2>&1 || die "curl is required"
  if command -v python3 >/dev/null 2>&1; then
    PYTHON_BIN="$(command -v python3)"
  elif command -v python >/dev/null 2>&1; then
    PYTHON_BIN="$(command -v python)"
  else
    die "python3 is required for JSON assertions"
  fi
  export PYTHON_BIN

  [[ "$CONCURRENCY" =~ ^[1-9][0-9]*$ ]] || die "CONCURRENCY must be a positive integer"
  [[ "$REQUESTS" =~ ^[1-9][0-9]*$ ]] || die "REQUESTS must be a positive integer"
}

json_string() {
  "$PYTHON_BIN" - "$1" <<'PY'
import json
import sys

print(json.dumps(sys.argv[1]))
PY
}

json_scalar() {
  "$PYTHON_BIN" - "$1" "$2" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    current = json.load(handle)

for part in sys.argv[2].split("."):
    if isinstance(current, list):
        current = current[int(part)]
    elif isinstance(current, dict):
        current = current.get(part)
    else:
        current = None
    if current is None:
        sys.exit(2)

if isinstance(current, bool):
    print("true" if current else "false")
else:
    print(current)
PY
}

json_list_field() {
  "$PYTHON_BIN" - "$1" "$2" "$3" "$4" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    items = json.load(handle)

match_field, match_value, return_field = sys.argv[2], sys.argv[3], sys.argv[4]
for item in items:
    if str(item.get(match_field, "")) == match_value:
        value = item.get(return_field)
        if value is not None:
            print(value)
        sys.exit(0)
sys.exit(2)
PY
}

first_active_market_field() {
  "$PYTHON_BIN" - "$1" "$2" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    payload = json.load(handle)

markets = payload.get("markets") or []
for market in markets:
    if market.get("status") == "active":
        print(market[sys.argv[2]])
        sys.exit(0)
if markets:
    print(markets[0][sys.argv[2]])
    sys.exit(0)
sys.exit(2)
PY
}

assert_health() {
  "$PYTHON_BIN" - "$1" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    payload = json.load(handle)

if payload.get("status") != "ok":
    sys.exit("health status is not ok")
if payload.get("db_ok") is not True:
    sys.exit("db_ok is not true")
if "redis_ok" in payload and payload.get("redis_ok") is not True:
    sys.exit("redis_ok is not true")
PY
}

assert_team_me() {
  "$PYTHON_BIN" - "$1" "$2" "$3" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    payload = json.load(handle)

team_slug = payload.get("team", {}).get("slug")
agent_slug = payload.get("agent", {}).get("slug")
if team_slug != sys.argv[2]:
    sys.exit(f"token belongs to unexpected team {team_slug!r}")
if agent_slug != sys.argv[3]:
    sys.exit(f"token belongs to unexpected agent {agent_slug!r}")
PY
}

assert_risk_rejection() {
  "$PYTHON_BIN" - "$1" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    payload = json.load(handle)

if payload.get("order", {}).get("status") != "rejected":
    sys.exit("order was not rejected")
if not payload.get("risk_event"):
    sys.exit("risk_event missing")
if payload.get("violation", {}).get("type") != "order_value_limit":
    sys.exit("expected order_value_limit violation")
PY
}

request() {
  local method="$1"
  local path="$2"
  local token="$3"
  local body="$4"
  local out_file="$5"
  local url="$path"

  if [[ "$url" != http://* && "$url" != https://* ]]; then
    url="$API_BASE$path"
  fi

  if [[ -n "$token" && -n "$body" ]]; then
    curl -sS -o "$out_file" -w "%{http_code}" -X "$method" -H "Accept: application/json" -H "Authorization: Bearer $token" -H "Content-Type: application/json" --data "$body" "$url"
  elif [[ -n "$token" ]]; then
    curl -sS -o "$out_file" -w "%{http_code}" -X "$method" -H "Accept: application/json" -H "Authorization: Bearer $token" "$url"
  elif [[ -n "$body" ]]; then
    curl -sS -o "$out_file" -w "%{http_code}" -X "$method" -H "Accept: application/json" -H "Content-Type: application/json" --data "$body" "$url"
  else
    curl -sS -o "$out_file" -w "%{http_code}" -X "$method" -H "Accept: application/json" "$url"
  fi
}

expect_status() {
  local actual="$1"
  local expected="$2"
  local label="$3"

  [[ "$actual" == "$expected" ]] || die "$label returned HTTP $actual, expected $expected"
  pass "$label"
}

admin_request() {
  request "$1" "$2" "$ARENA_ADMIN_TOKEN" "$3" "$4"
}

agent_request() {
  request "$1" "$2" "$agent_token" "$3" "$4"
}

write_agent_packet() {
  local token="$1"
  mkdir -p "$ACCESS_DIR"
  umask 077
  cat >"$TOKEN_FILE" <<EOF
ARENA_BASE_URL=$API_BASE
ARENA_API_TOKEN=$token
ARENA_TEAM_SLUG=$SMOKE_TEAM_SLUG
ARENA_AGENT_SLUG=$SMOKE_AGENT_SLUG
EOF
  chmod 600 "$TOKEN_FILE"
}

read_agent_packet_token() {
  [[ -f "$TOKEN_FILE" ]] || return 1
  local token_line
  token_line="$(grep -E '^ARENA_API_TOKEN=' "$TOKEN_FILE" | head -n 1 || true)"
  [[ -n "$token_line" ]] || return 1
  printf '%s' "$token_line" | sed 's/^ARENA_API_TOKEN=//'
}

cleanup() {
  local exit_code=$?
  set +e

  if [[ "$cleanup_ready" == "1" && -n "$round_id" && -n "$team_id" ]]; then
    request "POST" "/api/v1/admin/rounds/$round_id/teams/$team_id/reset" "$ARENA_ADMIN_TOKEN" "" "$tmp_dir/cleanup-reset.json" >/dev/null 2>&1
    request "POST" "/api/v1/admin/rounds/$round_id/teams/$team_id/withdraw" "$ARENA_ADMIN_TOKEN" "" "$tmp_dir/cleanup-withdraw.json" >/dev/null 2>&1
    request "POST" "/api/v1/admin/teams/$team_id/pause" "$ARENA_ADMIN_TOKEN" "" "$tmp_dir/cleanup-pause.json" >/dev/null 2>&1
    request "POST" "/api/v1/admin/rounds/$round_id/teams/$team_id/reset" "$ARENA_ADMIN_TOKEN" "" "$tmp_dir/cleanup-reset-final.json" >/dev/null 2>&1
    if [[ "$exit_code" == "0" ]]; then
      pass "cleanup reset, withdrew, and paused smoke team"
    else
      echo "INFO cleanup attempted for smoke team" >&2
    fi
  fi

  [[ -n "$tmp_dir" ]] && rm -rf "$tmp_dir"
  exit "$exit_code"
}

load_probe() {
  local failures="$tmp_dir/load-failures.txt"
  : >"$failures"

  local i path status
  for i in $(seq 1 "$REQUESTS"); do
    (
      case $((i % 3)) in
        0) path="/health" ;;
        1) path="/api/v1/leaderboard" ;;
        *) path="/api/v1/markets" ;;
      esac
      status="$(request "GET" "$path" "" "" "$tmp_dir/load-$i.json" 2>/dev/null || printf '000')"
      if [[ "$status" != "200" ]]; then
        printf '%s %s %s\n' "$i" "$path" "$status" >>"$failures"
      fi
    ) &

    if (( i % CONCURRENCY == 0 )); then
      wait
    fi
  done
  wait

  if [[ -s "$failures" ]]; then
    die "read-only load probe had $(wc -l <"$failures" | tr -d ' ') failures"
  fi
  pass "read-only load probe ($REQUESTS requests, concurrency $CONCURRENCY)"
}

main() {
  require_tools
  load_env_file

  API_BASE="$(printenv API_BASE 2>/dev/null || true)"
  FRONTEND_URL="$(printenv FRONTEND_URL 2>/dev/null || true)"
  if [[ -z "$API_BASE" ]]; then API_BASE="$(printenv NEXT_PUBLIC_ARENA_API_BASE_URL 2>/dev/null || true)"; fi
  if [[ -z "$FRONTEND_URL" ]]; then FRONTEND_URL="$(printenv ARENA_FRONTEND_ORIGIN 2>/dev/null || true)"; fi
  ARENA_ADMIN_TOKEN="$(printenv ARENA_ADMIN_TOKEN 2>/dev/null || true)"
  API_BASE="$(printf '%s' "$API_BASE" | sed 's:/*$::')"
  FRONTEND_URL="$(printf '%s' "$FRONTEND_URL" | sed 's:/*$::')"
  [[ -n "$API_BASE" ]] || die "NEXT_PUBLIC_ARENA_API_BASE_URL is required in $ENV_FILE"
  [[ -n "$FRONTEND_URL" ]] || die "ARENA_FRONTEND_ORIGIN is required in $ENV_FILE"
  [[ -n "$ARENA_ADMIN_TOKEN" ]] || die "ARENA_ADMIN_TOKEN is required in $ENV_FILE"

  tmp_dir="$(mktemp -d)"
  trap cleanup EXIT

  TOKEN_FILE="$ACCESS_DIR/$SMOKE_TEAM_SLUG-$SMOKE_AGENT_SLUG-access.txt"
  export TOKEN_FILE

  local status body headers
  body="$tmp_dir/health.json"
  status="$(request "GET" "/health" "" "" "$body")"
  expect_status "$status" "200" "backend health"
  assert_health "$body"

  body="$tmp_dir/frontend-root.html"
  status="$(curl -sS -o "$body" -w "%{http_code}" "$FRONTEND_URL/")"
  expect_status "$status" "200" "frontend root"

  body="$tmp_dir/frontend-onboard.html"
  status="$(curl -sS -o "$body" -w "%{http_code}" "$FRONTEND_URL/onboard")"
  expect_status "$status" "200" "frontend onboard page"

  body="$tmp_dir/leaderboard.json"
  status="$(request "GET" "/api/v1/leaderboard" "" "" "$body")"
  expect_status "$status" "200" "public leaderboard"
  round_id="$(json_scalar "$body" "round.id")"
  local round_slug require_locked_agents
  round_slug="$(json_scalar "$body" "round.slug")"
  require_locked_agents="$(json_scalar "$body" "round.require_locked_agents")"

  body="$tmp_dir/markets.json"
  status="$(request "GET" "/api/v1/markets" "" "" "$body")"
  expect_status "$status" "200" "public markets"
  local market_id yes_price limit_price estimate_bps
  market_id="$(first_active_market_field "$body" "id")"
  yes_price="$(first_active_market_field "$body" "yes_price_bps")"
  limit_price="$yes_price"
  if (( limit_price < 1 )); then
    limit_price=1
  elif (( limit_price > 9999 )); then
    limit_price=9999
  fi
  estimate_bps=$((limit_price + 250))
  if (( estimate_bps > 9999 )); then
    estimate_bps=9999
  fi

  body="$tmp_dir/me-unauthorized.json"
  status="$(request "GET" "/api/v1/me" "" "" "$body")"
  expect_status "$status" "401" "agent auth rejects missing token"

  body="$tmp_dir/cors.body"
  headers="$tmp_dir/cors.headers"
  status="$(curl -sS -X OPTIONS \
    -H "Origin: $FRONTEND_URL" \
    -H "Access-Control-Request-Method: GET" \
    -H "Access-Control-Request-Headers: Authorization, Content-Type" \
    -D "$headers" \
    -o "$body" \
    -w "%{http_code}" \
    "$API_BASE/api/v1/me")"
  expect_status "$status" "204" "CORS preflight"
  "$PYTHON_BIN" - "$headers" "$FRONTEND_URL" <<'PY'
import sys

headers = {}
with open(sys.argv[1], encoding="utf-8", errors="replace") as handle:
    for line in handle:
        if ":" not in line:
            continue
        key, value = line.split(":", 1)
        headers[key.strip().lower()] = value.strip()

if headers.get("access-control-allow-origin") != sys.argv[2]:
    sys.exit("unexpected CORS allow-origin")
PY

  for endpoint in health summary rounds teams markets; do
    body="$tmp_dir/admin-$endpoint.json"
    status="$(admin_request "GET" "/api/v1/admin/$endpoint" "" "$body")"
    expect_status "$status" "200" "admin $endpoint"
  done

  local slug_json name_json agent_id agent_status agent_token
  slug_json="$(json_string "$SMOKE_TEAM_SLUG")"
  name_json="$(json_string "$SMOKE_TEAM_NAME")"
  body="$tmp_dir/admin-teams-for-smoke.json"
  status="$(admin_request "GET" "/api/v1/admin/teams" "" "$body")"
  expect_status "$status" "200" "smoke team lookup"
  team_id="$(json_list_field "$body" "slug" "$SMOKE_TEAM_SLUG" "id" || true)"
  if [[ -z "$team_id" ]]; then
    body="$tmp_dir/admin-create-smoke-team.json"
    status="$(admin_request "POST" "/api/v1/admin/teams" "{\"slug\":$slug_json,\"name\":$name_json}" "$body")"
    expect_status "$status" "201" "smoke team created"
    team_id="$(json_scalar "$body" "id")"
  else
    pass "smoke team reused"
  fi
  cleanup_ready=1

  body="$tmp_dir/admin-resume-smoke-team.json"
  status="$(admin_request "POST" "/api/v1/admin/teams/$team_id/resume" "" "$body")"
  expect_status "$status" "200" "smoke team resumed"

  body="$tmp_dir/admin-enroll-smoke-team.json"
  status="$(admin_request "POST" "/api/v1/admin/rounds/$round_id/teams/$team_id/enroll" "" "$body")"
  expect_status "$status" "200" "smoke team enrolled in $round_slug"

  body="$tmp_dir/admin-agents-for-smoke.json"
  status="$(admin_request "GET" "/api/v1/admin/teams/$team_id/agents" "" "$body")"
  expect_status "$status" "200" "smoke agent lookup"
  agent_id="$(json_list_field "$body" "slug" "$SMOKE_AGENT_SLUG" "id" || true)"
  agent_status=""
  if [[ -n "$agent_id" ]]; then
    agent_status="$(json_list_field "$body" "slug" "$SMOKE_AGENT_SLUG" "status" || true)"
  fi

  if [[ -z "$agent_id" ]]; then
    local agent_slug_json agent_name_json
    agent_slug_json="$(json_string "$SMOKE_AGENT_SLUG")"
    agent_name_json="$(json_string "$SMOKE_AGENT_NAME")"
    body="$tmp_dir/admin-create-smoke-agent.json"
    status="$(admin_request "POST" "/api/v1/admin/teams/$team_id/agents" "{\"slug\":$agent_slug_json,\"name\":$agent_name_json,\"kind\":\"agent\",\"metadata\":{\"purpose\":\"fly_pilot_gate\"}}" "$body")"
    expect_status "$status" "201" "smoke agent created"
    agent_id="$(json_scalar "$body" "agent.id")"
    agent_token="$(json_scalar "$body" "api_token")"
    write_agent_packet "$agent_token"
  else
    pass "smoke agent reused"
    if [[ "$agent_status" != "active" ]]; then
      body="$tmp_dir/admin-resume-smoke-agent.json"
      status="$(admin_request "POST" "/api/v1/admin/agents/$agent_id/resume" "" "$body")"
      expect_status "$status" "200" "smoke agent resumed"
    fi
    agent_token="$(read_agent_packet_token || true)"
    if [[ -z "$agent_token" ]]; then
      body="$tmp_dir/admin-rotate-smoke-agent.json"
      status="$(admin_request "POST" "/api/v1/admin/agents/$agent_id/rotate-token" "" "$body")"
      expect_status "$status" "200" "smoke agent token rotated into local packet"
      agent_token="$(json_scalar "$body" "api_token")"
      write_agent_packet "$agent_token"
    fi
  fi

  body="$tmp_dir/smoke-me.json"
  status="$(agent_request "GET" "/api/v1/me" "" "$body" || true)"
  if [[ "$status" != "200" ]]; then
    body="$tmp_dir/admin-rotate-invalid-smoke-agent.json"
    status="$(admin_request "POST" "/api/v1/admin/agents/$agent_id/rotate-token" "" "$body")"
    expect_status "$status" "200" "smoke agent token refreshed"
    agent_token="$(json_scalar "$body" "api_token")"
    write_agent_packet "$agent_token"
    body="$tmp_dir/smoke-me-after-rotate.json"
    status="$(agent_request "GET" "/api/v1/me" "" "$body")"
  fi
  expect_status "$status" "200" "agent /me with smoke token"
  assert_team_me "$body" "$SMOKE_TEAM_SLUG" "$SMOKE_AGENT_SLUG"

  if [[ "$require_locked_agents" == "true" ]]; then
    body="$tmp_dir/admin-lock-smoke-agent.json"
    status="$(admin_request "POST" "/api/v1/admin/rounds/$round_id/agents/$agent_id/lock" "{\"commit_sha\":\"fly-pilot-gate\",\"docker_image\":\"fly-pilot-gate\",\"locked_by\":\"fly_pilot_gate\",\"confirm\":\"replace_active_round_lock\"}" "$body")"
    expect_status "$status" "200" "smoke agent locked for $round_slug"
  fi

  body="$tmp_dir/admin-reset-smoke-team.json"
  status="$(admin_request "POST" "/api/v1/admin/rounds/$round_id/teams/$team_id/reset" "" "$body")"
  expect_status "$status" "200" "smoke team reset before mutations"

  body="$tmp_dir/smoke-heartbeat.json"
  status="$(agent_request "POST" "/api/v1/heartbeat" "{\"status\":\"online\",\"metadata\":{\"source\":\"fly_pilot_gate\"}}" "$body")"
  expect_status "$status" "201" "agent heartbeat"

  body="$tmp_dir/smoke-portfolio.json"
  status="$(agent_request "GET" "/api/v1/portfolio" "" "$body")"
  expect_status "$status" "200" "agent portfolio"

  body="$tmp_dir/smoke-fills.json"
  status="$(agent_request "GET" "/api/v1/fills" "" "$body")"
  expect_status "$status" "200" "agent fills"

  body="$tmp_dir/smoke-valid-order.json"
  status="$(agent_request "POST" "/api/v1/orders" "{\"market_id\":$market_id,\"outcome\":\"yes\",\"action\":\"buy\",\"amount_cents\":1000,\"limit_price_bps\":$limit_price,\"estimated_probability_bps\":$estimate_bps,\"confidence\":\"medium\",\"reason\":\"Fly pilot gate valid order\"}" "$body")"
  expect_status "$status" "201" "small valid order"
  json_scalar "$body" "order.status" >/dev/null || die "valid order response missing status"

  body="$tmp_dir/smoke-risk-reject.json"
  status="$(agent_request "POST" "/api/v1/orders" "{\"market_id\":$market_id,\"outcome\":\"yes\",\"action\":\"buy\",\"amount_cents\":99999999,\"limit_price_bps\":$limit_price,\"estimated_probability_bps\":$estimate_bps,\"confidence\":\"medium\",\"reason\":\"Fly pilot gate risk rejection\"}" "$body")"
  expect_status "$status" "400" "intentional risk rejection"
  assert_risk_rejection "$body"

  body="$tmp_dir/admin-summary-after-smoke.json"
  status="$(admin_request "GET" "/api/v1/admin/summary" "" "$body")"
  expect_status "$status" "200" "admin summary after smoke order"

  load_probe

  pass "fly pilot gate passed against $API_BASE"
}

main "$@"
