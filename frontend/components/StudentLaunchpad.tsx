"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import { apiBase, formatAPIError, getMe } from "@/lib/api";
import type { MeResponse } from "@/lib/types";

const sessionTokenKey = "prediction-agent-arena.student-token";

export function StudentLaunchpad() {
  const [token, setToken] = useState(() => readSessionToken());
  const [rememberForTab, setRememberForTab] = useState(() => Boolean(readSessionToken()));
  const [identity, setIdentity] = useState<MeResponse | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [copied, setCopied] = useState("");

  useEffect(() => {
    if (!rememberForTab) {
      window.sessionStorage.removeItem(sessionTokenKey);
      return;
    }
    if (token) {
      window.sessionStorage.setItem(sessionTokenKey, token);
    } else {
      window.sessionStorage.removeItem(sessionTokenKey);
    }
  }, [rememberForTab, token]);

  const commands = useMemo(() => buildCommands(apiBase, token), [token]);

  async function verify(event?: FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    if (!token.trim()) {
      setError("ARENA_API_TOKEN is required");
      setIdentity(null);
      return;
    }
    setLoading(true);
    setError("");
    setCopied("");
    try {
      const next = await getMe(token.trim());
      setIdentity(next);
    } catch (err) {
      setIdentity(null);
      setError(formatAPIError(err));
    } finally {
      setLoading(false);
    }
  }

  async function copy(label: string, value: string) {
    try {
      await window.navigator.clipboard.writeText(value);
      setCopied(label);
    } catch {
      setCopied("copy failed");
    }
  }

  return (
    <div className="stack">
      <section className="form-band stack">
        <form className="form-grid" onSubmit={verify}>
          <label>
            Agent API token
            <input
              value={token}
              onChange={(event) => setToken(event.target.value)}
              type="password"
              placeholder="paa_agent_..."
              autoComplete="off"
            />
          </label>
          <label className="checkbox-line">
            <input checked={rememberForTab} onChange={(event) => setRememberForTab(event.target.checked)} type="checkbox" />
            Remember for this browser tab
          </label>
          <div className="actions">
            <button className="primary" type="submit" disabled={loading}>
              {loading ? "Verifying" : "Verify token"}
            </button>
            <button
              type="button"
              onClick={() => {
                setToken("");
                setIdentity(null);
                setRememberForTab(false);
              }}
            >
              Clear
            </button>
          </div>
        </form>
        {error ? <div className="notice error">{error}</div> : null}
        {copied ? <div className="notice">{copied === "copy failed" ? copied : `Copied ${copied}`}</div> : null}
      </section>

      {identity ? (
        <section className="grid">
          <div className="metric">
            <span className="label">Team</span>
            <span className="value compact">{identity.team.name}</span>
            <span className="muted">{identity.team.slug}</span>
          </div>
          <div className="metric">
            <span className="label">Agent</span>
            <span className="value compact">{identity.agent?.name ?? "legacy token"}</span>
            <span className={`status ${identity.agent?.status ?? "active"}`}>{identity.agent?.status ?? "legacy"}</span>
          </div>
          <div className="metric">
            <span className="label">Active Round</span>
            <span className="value compact">{identity.active_round?.name ?? "none"}</span>
            <span className={`status ${identity.active_round?.status ?? "paused"}`}>{identity.active_round?.status ?? "none"}</span>
          </div>
          <div className="metric">
            <span className="label">Auth Mode</span>
            <span className="value compact">{identity.legacy_team_auth ? "legacy team token" : "registered agent"}</span>
            <span className="muted">student API</span>
          </div>
        </section>
      ) : null}

      <section className="split">
        <div className="panel stack">
          <h2>Commands</h2>
          <CommandBlock label="Verify identity" value={commands.me} onCopy={copy} />
          <CommandBlock label="Send heartbeat" value={commands.heartbeat} onCopy={copy} />
          <CommandBlock label="Run random agent" value={commands.randomAgent} onCopy={copy} />
          <CommandBlock label="Install SDK editable" value={commands.installSDK} onCopy={copy} />
        </div>
        <div className="panel stack">
          <h2>Common Errors</h2>
          <ul className="compact-list">
            <li>
              <strong>401</strong> missing or invalid token.
            </li>
            <li>
              <strong>403</strong> paused team, paused/revoked agent, or locked-round mismatch.
            </li>
            <li>
              <strong>409</strong> no active round, paused round, or final-round state conflict.
            </li>
            <li>
              <strong>429</strong> slow your loop down and retry with backoff.
            </li>
            <li>
              <strong>risk rejection</strong> check order size, reason, probability, cash, positions, and exposure.
            </li>
          </ul>
          <h2>Default Loop Pace</h2>
          <p className="muted">Heartbeat every 10-30 seconds and submit orders slowly enough to stay under route and risk limits.</p>
        </div>
      </section>
    </div>
  );
}

function readSessionToken() {
  if (typeof window === "undefined") {
    return "";
  }
  return window.sessionStorage.getItem(sessionTokenKey) ?? "";
}

function CommandBlock({ label, value, onCopy }: { label: string; value: string; onCopy: (label: string, value: string) => void }) {
  return (
    <div className="command-block">
      <div className="inline-row">
        <strong>{label}</strong>
        <button type="button" onClick={() => void onCopy(label, value)}>
          Copy
        </button>
      </div>
      <pre>{value}</pre>
    </div>
  );
}

function buildCommands(baseURL: string, token: string) {
  const trimmedToken = token.trim();
  const displayToken = trimmedToken || "$ARENA_API_TOKEN";
  const envToken = trimmedToken ? shellQuote(trimmedToken) : "$ARENA_API_TOKEN";
  const quotedBase = shellQuote(baseURL);
  return {
    me: `curl -sS -H "Authorization: Bearer ${displayToken}" ${baseURL}/api/v1/me`,
    heartbeat: `curl -sS -X POST -H "Authorization: Bearer ${displayToken}" -H "Content-Type: application/json" ${baseURL}/api/v1/heartbeat -d '{"status":"online","metadata":{"source":"student-launchpad"}}'`,
    randomAgent: `ARENA_BASE_URL=${quotedBase} ARENA_API_TOKEN=${envToken} ARENA_MAX_RETRIES=2 PYTHONPATH=sdk/python python examples/python-random-agent/agent.py`,
    installSDK: "python -m pip install -e sdk/python",
  };
}

function shellQuote(value: string) {
  return `'${value.replaceAll("'", "'\"'\"'")}'`;
}
