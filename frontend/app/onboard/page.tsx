import Link from "next/link";
import { apiBase, formatAPIError, getLeaderboard, isNoActiveRoundError } from "@/lib/api";
import type { Round } from "@/lib/types";

export const dynamic = "force-dynamic";

const discordInviteURL = process.env.NEXT_PUBLIC_DISCORD_INVITE_URL?.trim();

const setupCommand = [
  "export ARENA_BASE_URL=" + apiBase,
  "export ARENA_API_TOKEN=paa_agent_...",
  "PYTHONPATH=sdk/python mise exec -- python examples/python-random-agent/agent.py",
].join("\n");

const heartbeatCommand = [
  "curl -sS \\",
  '  -H "Authorization: Bearer $ARENA_API_TOKEN" \\',
  "  " + apiBase + "/api/v1/me",
].join("\n");

export default async function OnboardPage() {
	const leaderboard = await getLeaderboard()
		.then((value) => ({ data: value, error: "", noActiveRound: false }))
		.catch((error: unknown) =>
			isNoActiveRoundError(error) ? { data: null, error: "", noActiveRound: true } : { data: null, error: formatAPIError(error), noActiveRound: false },
		);
	const activeRound = leaderboard.data?.round ?? null;
	const currentMode = describeCurrentMode(activeRound, leaderboard.error);

  return (
    <div className="stack">
      <section className="page-head">
        <div>
          <h1>Arena Onboarding</h1>
          <p className="muted">Practice agents first, then join the timed contest with a locked official agent.</p>
        </div>
        <div className="actions">
          <Link className="button" href="/agent">
            Verify token
          </Link>
          <Link className="button primary" href="/leaderboard">
            Open leaderboard
          </Link>
          {discordInviteURL ? (
            <a className="button" href={discordInviteURL} rel="noreferrer" target="_blank">
              Join Discord
            </a>
          ) : null}
        </div>
      </section>

			{leaderboard.noActiveRound ? <div className="notice">No active round is currently running. Players can onboard now and start agents after the operator activates a round.</div> : null}
			{leaderboard.error ? <div className="notice error">Backend API unavailable: {leaderboard.error}</div> : null}

      <section className="grid">
        <div className="metric">
          <span className="label">API Base</span>
          <span className="value compact">{apiBase}</span>
        </div>
        <div className="metric">
          <span className="label">Active Round</span>
					<span className="value compact">{activeRound ? activeRound.slug + " / " + activeRound.status : leaderboard.noActiveRound ? "none" : "unavailable"}</span>
        </div>
        <div className="metric">
          <span className="label">Practice Signup</span>
          <span className="value compact">Discord pinned link</span>
        </div>
        <div className="metric">
          <span className="label">Contest Entry</span>
          <span className="value compact">Timed signup + locked agent</span>
        </div>
      </section>

      <section className="split">
        <div className="panel stack">
          <h2>Current Mode</h2>
          <div className={"readiness-item " + currentMode.tone}>
            <strong>{currentMode.label}</strong>
            <span>{currentMode.detail}</span>
          </div>
        </div>

        <div className="panel stack">
          <h2>Discord Lobby</h2>
          {discordInviteURL ? (
            <>
              <p className="muted">Use Discord for practice signup, contest signup windows, schedule changes, and support.</p>
              <a className="button primary" href={discordInviteURL} rel="noreferrer" target="_blank">
                Open Discord
              </a>
            </>
          ) : (
            <p className="muted">Discord invite and signup links are shared privately by the operator.</p>
          )}
          <div className="notice">Practice and contest signup links belong in pinned Discord posts, not in public repo files or support screenshots.</div>
        </div>
      </section>

      <section className="split">
        <div className="panel stack">
          <h2>Practice Track</h2>
          <ol className="compact-list">
            <li>Join Discord and use the pinned practice signup link.</li>
            <li>
              Get a private <code>paa_agent_...</code> token and verify it on the agent launchpad.
            </li>
            <li>Run one example agent locally before editing strategy code.</li>
            <li>Iterate freely against synthetic/fake-market data; practice leaderboard scores are informal.</li>
            <li>
              Keep loops paced: heartbeat every 20-30 seconds and back off on <code>429</code>.
            </li>
          </ol>
          <div className="actions">
            <Link className="button" href="/agent">
              Agent launchpad
            </Link>
            <Link className="button" href="/leaderboard">
              Practice leaderboard
            </Link>
          </div>
        </div>

        <div className="panel stack">
          <h2>Contest Track</h2>
          <ol className="compact-list">
            <li>Watch Discord for the timed contest signup window and lock deadline.</li>
            <li>Register the team before the window closes; late entries are operator-approved only.</li>
            <li>Submit or confirm the official agent that should be locked for the contest round.</li>
            <li>Start local loops only after the operator announces the contest round is active.</li>
            <li>Use the official leaderboard after the contest is completed, frozen, and exported.</li>
          </ol>
          <div className="actions">
            <Link className="button primary" href="/leaderboard/evaluation">
              Contest view
            </Link>
            <Link className="button" href="/leaderboard">
              Leaderboard
            </Link>
          </div>
        </div>
      </section>

      <section className="split">
        <div className="panel stack">
          <h2>Local Agent Command</h2>
          <p className="muted">Use this shape from the repository root after replacing the token.</p>
          <div className="command-block">
            <pre>{setupCommand}</pre>
          </div>
        </div>

        <div className="panel stack">
          <h2>Credential Check</h2>
          <p className="muted">A valid token should return your team, agent, and active round.</p>
          <div className="command-block">
            <pre>{heartbeatCommand}</pre>
          </div>
        </div>
      </section>

      <section className="split">
        <div className="panel stack">
          <h2>Common Responses</h2>
          <ul className="compact-list">
            <li>
              <code>401</code>: token missing, malformed, rotated, or pasted with extra characters.
            </li>
            <li>
              <code>403</code>: team or registered agent is paused, revoked, not enrolled, or not locked for contest mode.
            </li>
            <li>
              <code>404</code>: no active round or the market is not available.
            </li>
            <li>
              <code>400</code>: invalid order shape or risk policy rejection.
            </li>
          </ul>
        </div>

        <div className="panel stack">
          <h2>Safety Boundaries</h2>
          <div className="readiness-grid">
            <div className="readiness-item ok">
              <strong>Paper trading only</strong>
              <span>No wallets, exchange keys, or real-money orders.</span>
            </div>
            <div className="readiness-item ok">
              <strong>Private credentials</strong>
              <span>Keep tokens out of Discord channels, commits, screenshots, and logs.</span>
            </div>
            <div className="readiness-item ok">
              <strong>Official contest lock</strong>
              <span>Contest entries use the agent approved before the signup window closes.</span>
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}

function describeCurrentMode(round: Round | null, error: string): { label: string; detail: string; tone: "ok" | "warn" | "error" } {
  if (error) {
    return {
      label: "Status unavailable",
      detail: "The onboarding page could not read the active round. Check the API before inviting players.",
      tone: "error",
    };
  }
  if (!round) {
    return {
      label: "No active round",
      detail: "Players can read onboarding, but agents need an active round before they can trade.",
      tone: "warn",
    };
  }
  if (round.status === "paused") {
    return {
      label: "Round paused",
      detail: round.slug + " is paused. Stop local loops until the operator posts a resume notice.",
      tone: "warn",
    };
  }
  if (round.status === "completed") {
    return {
      label: "Round completed",
      detail: round.slug + " is complete. Wait for frozen results or the next practice/contest announcement.",
      tone: "warn",
    };
  }
  if (round.status === "active" && (round.mode === "replay" || round.require_locked_agents)) {
    return {
      label: "Contest mode active",
      detail: round.slug + " requires locked official agents. Practice tokens that are not locked will be rejected for mutations.",
      tone: "warn",
    };
  }
  if (round.status === "active" && round.mode === "practice") {
    return {
      label: "Practice is open",
      detail: round.slug + " accepts active enrolled registered agents for ad-hoc synthetic-data testing.",
      tone: "ok",
    };
  }
  return {
    label: round.mode + " / " + round.status,
    detail: round.slug + " is the selected round. Follow Discord announcements before starting local loops.",
    tone: "warn",
  };
}
