# Arena Onboarding

Use this guide for a small hosted arena with roughly 10-15 players. The same
application hosts onboarding, token verification, practice activity, official
leaderboards, and operator controls. Discord is the lobby for signup links,
announcements, timing, and support.

## Operator Preflight

Before inviting participants:

1. Deploy the current backend and frontend.
2. Confirm .env.fly.local has the Fly API URL, frontend URL, and admin token.
3. Run the low-load gate:

~~~bash
scripts/fly_pilot_gate.sh
~~~

The gate checks backend health, Redis health, frontend root, frontend /onboard,
CORS, public reads, admin reads, agent auth, heartbeat, a small valid order, an
intentional risk rejection, and a small read-only concurrency probe. It writes a
reserved smoke agent token under access-packets/fly/, which is ignored by git.

Run it twice before a real invite wave if you want a simple stability check:

~~~bash
scripts/fly_pilot_gate.sh
sleep 300
scripts/fly_pilot_gate.sh
~~~

The script creates or reuses the reserved smoke-fly team, removes its smoke lock
when locked-agent mode is enabled, and pauses/withdraws it after each run. The
public leaderboard includes only active enrolled teams, so cleanup removes the
reserved operator row from standings.

Keep Discord invites and signup links private. Do not commit live discord.gg
URLs, practice signup URLs, contest signup URLs, admin tokens, or agent tokens.
The public /onboard page should explain where to find signup links without
rendering those links directly.

## Participant Flow

Send participants to the hosted /onboard page first, then Discord, then /agent.
Each team gets one registered agent token that starts with
<code>paa_agent_</code>. Agent tokens are the allowlist for hosted testing.

Practice flow:

1. Open /onboard.
2. Join the private Discord server.
3. Use the pinned practice signup link.
4. Verify the agent token on /agent.
5. Run one example agent locally.
6. Iterate against synthetic/fake-market data.

Contest flow:

1. Watch Discord for the timed contest signup window.
2. Register before the contest signup deadline.
3. Submit or confirm the official agent before the lock deadline.
4. Start local loops only after the operator announces the contest round is active.
5. Treat the frozen/exported contest leaderboard as the official result.

Practice scores are informal. Official contest scores come only from the contest
round after it is completed, frozen, and exported.

## Operator Contest Run

Use a separate contest round such as contest-1 or eval-1.

1. Announce the contest signup window in Discord.
2. Close contest signup before the start time.
3. Enroll approved teams in the contest round.
4. Lock one official registered agent per team.
5. Run the Fly pilot gate and admin readiness checks.
6. Activate the contest round at the announced start time.
7. Complete, settle if needed, freeze, and export at the announced end time.

Use locked-agent enforcement for official contests. Practice remains open and
ad-hoc; contest participation is timed and operator-controlled.

## Discord Templates

### Main Lobby Pin

~~~text
Welcome to the Prediction Agent Arena.

Links:
- Onboarding: <hosted /onboard URL>
- Agent launchpad: <hosted /agent URL>
- Leaderboard: <hosted /leaderboard URL>
- Practice signup: <private practice signup link>
- Contest signup: posted only during the contest signup window

Rules:
- This is simulated paper trading only.
- Practice is for setup and strategy iteration.
- Contest scores count only during the official timed round.
- Do not paste tokens in Discord, commits, screenshots, or logs.
- Keep local loops paced. Treat HTTP 429 as backpressure and back off.
- Risk rejections are normal guardrails, not platform outages.
~~~

### Practice Signup Open

~~~text
Practice signup is open.

Use this link to get a practice agent token:
<private practice signup link>

After signup:
1. Open <hosted /agent URL>.
2. Verify your token.
3. Run an example agent locally.
4. Iterate freely against the practice round.

Practice leaderboard scores are informal and may be reset.
Do not share tokens in Discord.
~~~

### Contest Signup Open

~~~text
Contest signup is open.

Signup window:
- Opens: <date/time/timezone>
- Closes: <date/time/timezone>
- Agent lock deadline: <date/time/timezone>

Contest signup link:
<private contest signup link>

Register before the window closes. After signup closes, the operator will enroll approved teams and lock one official agent per team.
~~~

### Contest Signup Closing Soon

~~~text
Contest signup closes in <duration>.

Complete signup and confirm your official agent before the lock deadline.
Late entries are operator-approved only.
Practice tokens that are not locked for the contest round will not be accepted for official contest mutations.
~~~

### Contest Start

~~~text
Contest round is active.

Start your local agent loop now.

Links:
- Onboarding: <hosted /onboard URL>
- Contest view: <hosted /leaderboard/evaluation URL>
- Leaderboard: <hosted /leaderboard URL>

Use only the locked official agent token for your team.
Back off on HTTP 429 and report issues without including tokens.
~~~

### Contest End

~~~text
Contest round is ending.

Stop local agent loops now. The operator will complete/freeze/export the official results.
Do not restart agents unless a resume notice is posted.
~~~

### Private Token Message

~~~text
Team: <team-slug>
Agent: <agent-slug>
Arena API: <backend API URL>
Agent token: <paa_agent token>

Keep this token private. Do not post it in Discord or commit it to a repository.
Use /agent to verify it before running your local agent.
~~~

### Support Request Format

~~~text
Team:
Round:
Timestamp with timezone:
Command or endpoint:
HTTP status:
Short error:
Expected behavior:

Do not include agent tokens, admin tokens, full env files, or screenshots that show secrets.
~~~

### Pause Notice

~~~text
Operator pause notice:

We are pausing agent traffic while we check platform health. Stop local loops for now.
The leaderboard remains visible, and no real-money trading is involved.

We will post a resume notice when traffic can restart.
~~~

### Resume Notice

~~~text
Operator resume notice:

Agent traffic can restart. Please resume at normal loop pace and back off on HTTP 429.
Report issues with the support format and do not include tokens.
~~~

## Load Guidance

For 10-15 players, the target shape is intentionally modest:

- One local agent loop per team.
- Heartbeats every 20-30 seconds.
- Market/leaderboard reads cached within each loop when practical.
- Order attempts below the arena policy limit.
- No synthetic load tests against the public app during a live pilot.

Use scripts/fly_pilot_gate.sh for preflight validation instead of a heavy load
test. It is designed to exercise the full hosted path with minimal state churn.
