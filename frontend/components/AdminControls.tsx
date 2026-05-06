"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { apiBase, formatMoney, getAdminSummary } from "@/lib/api";
import type { AdminSummary } from "@/lib/types";

type Message = { type: "ok" | "error"; text: string };

function formatDate(value?: string) {
  if (!value) {
    return "-";
  }
  return new Date(value).toLocaleString();
}

function parseError(text: string, fallback: string) {
  try {
    const parsed = JSON.parse(text) as { error?: { code?: string; message?: string } };
    if (parsed.error?.message) {
      return parsed.error.code ? `${parsed.error.code}: ${parsed.error.message}` : parsed.error.message;
    }
  } catch {
    // Plain-text fallback from older tooling.
  }
  return text || fallback;
}

export function AdminControls() {
  const [token, setToken] = useState("");
  const [message, setMessage] = useState<Message | null>(null);
  const [summary, setSummary] = useState<AdminSummary | null>(null);
  const [teamSlug, setTeamSlug] = useState("");
  const [teamName, setTeamName] = useState("");
  const [roundID, setRoundID] = useState("");
  const [roundSlug, setRoundSlug] = useState("");
  const [roundName, setRoundName] = useState("");
  const [loading, setLoading] = useState(false);

  const selectedRoundID = useMemo(() => {
    if (roundID) {
      return roundID;
    }
    return String(summary?.active_round?.id ?? summary?.latest_round?.id ?? "");
  }, [roundID, summary]);

  const refresh = useCallback(async () => {
    if (!token) {
      setMessage({ type: "error", text: "admin token is required" });
      return;
    }
    setLoading(true);
    setMessage(null);
    try {
      const nextSummary = await getAdminSummary(token);
      setSummary(nextSummary);
      if (!roundID) {
        const inferredID = nextSummary.active_round?.id ?? nextSummary.latest_round?.id;
        if (inferredID) {
          setRoundID(String(inferredID));
        }
      }
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "refresh failed" });
    } finally {
      setLoading(false);
    }
  }, [roundID, token]);

  useEffect(() => {
    if (!token) {
      return;
    }
    const timer = window.setInterval(() => {
      void refresh();
    }, 7000);
    return () => window.clearInterval(timer);
  }, [refresh, token]);

  async function request(path: string, options: RequestInit = {}) {
    setMessage(null);
    const response = await fetch(`${apiBase}${path}`, {
      ...options,
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
        ...(options.headers ?? {}),
      },
    });
    const text = await response.text();
    if (!response.ok) {
      throw new Error(parseError(text, `HTTP ${response.status}`));
    }
    await refresh();
    return text;
  }

  async function createTeam(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    try {
      const text = await request("/api/v1/admin/teams", {
        method: "POST",
        body: JSON.stringify({ slug: teamSlug, name: teamName || teamSlug }),
      });
      setMessage({ type: "ok", text });
      setTeamSlug("");
      setTeamName("");
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "create team failed" });
    }
  }

  async function createRound(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    try {
      const slug = roundSlug || `practice-${Date.now().toString().slice(-5)}`;
      const text = await request("/api/v1/admin/rounds", {
        method: "POST",
        body: JSON.stringify({
          slug,
          name: roundName || slug,
          mode: "practice",
          status: "draft",
          initial_balance_cents: 1000000,
        }),
      });
      setMessage({ type: "ok", text });
      setRoundSlug("");
      setRoundName("");
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "create round failed" });
    }
  }

  async function roundAction(action: "activate" | "pause" | "complete" | "reset" | "freeze-leaderboard") {
    if (!selectedRoundID) {
      setMessage({ type: "error", text: "round id is required" });
      return;
    }
    try {
      const text = await request(`/api/v1/admin/rounds/${selectedRoundID}/${action}`, { method: "POST" });
      setMessage({ type: "ok", text });
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : `${action} failed` });
    }
  }

  async function exportRound() {
    if (!selectedRoundID) {
      setMessage({ type: "error", text: "round id is required" });
      return;
    }
    try {
      const text = await request(`/api/v1/admin/export/${selectedRoundID}`, { method: "GET" });
      setMessage({ type: "ok", text });
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "export failed" });
    }
  }

  async function teamAction(teamID: number, action: "pause" | "resume" | "reset") {
    try {
      const text = await request(`/api/v1/admin/teams/${teamID}/${action}`, { method: "POST" });
      setMessage({ type: "ok", text });
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : `${action} failed` });
    }
  }

  return (
    <div className="stack">
      <section className="form-band stack">
        <div className="section-head">
          <div>
            <h2>Access</h2>
            <p className="muted">Active round: {summary?.active_round ? `${summary.active_round.name} (${summary.active_round.status})` : "none"}</p>
          </div>
          <button type="button" className="primary" onClick={() => void refresh()} disabled={loading}>
            {loading ? "Refreshing" : "Refresh"}
          </button>
        </div>
        <label>
          Admin token
          <input value={token} onChange={(event) => setToken(event.target.value)} type="password" placeholder="dev-admin-token" />
        </label>
        {message ? <pre className={`notice ${message.type === "error" ? "error" : ""}`}>{message.text}</pre> : null}
      </section>

      <div className="split">
        <section className="form-band stack">
          <h2>Round Controls</h2>
          <label>
            Round ID
            <input value={roundID} onChange={(event) => setRoundID(event.target.value)} inputMode="numeric" placeholder={selectedRoundID || "round id"} />
          </label>
          <div className="actions">
            <button type="button" className="primary" onClick={() => void roundAction("activate")}>
              Activate
            </button>
            <button type="button" onClick={() => void roundAction("pause")}>
              Pause
            </button>
            <button type="button" onClick={() => void roundAction("complete")}>
              Complete
            </button>
            <button type="button" onClick={() => void roundAction("reset")}>
              Reset
            </button>
            <button type="button" onClick={() => void roundAction("freeze-leaderboard")}>
              Freeze
            </button>
            <button type="button" onClick={() => void exportRound()}>
              Export
            </button>
          </div>
        </section>

        <section className="form-band stack">
          <h2>Create Round</h2>
          <form className="form-grid" onSubmit={createRound}>
            <label>
              Slug
              <input value={roundSlug} onChange={(event) => setRoundSlug(event.target.value)} placeholder="practice-2" />
            </label>
            <label>
              Name
              <input value={roundName} onChange={(event) => setRoundName(event.target.value)} placeholder="Practice Round 2" />
            </label>
            <button className="primary" type="submit">
              Create Round
            </button>
          </form>
        </section>
      </div>

      <section className="form-band stack">
        <h2>Create Team</h2>
        <form className="form-grid" onSubmit={createTeam}>
          <label>
            Slug
            <input value={teamSlug} onChange={(event) => setTeamSlug(event.target.value)} required />
          </label>
          <label>
            Name
            <input value={teamName} onChange={(event) => setTeamName(event.target.value)} />
          </label>
          <button className="primary" type="submit">
            Create Team
          </button>
        </form>
      </section>

      <section className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Team</th>
              <th>Status</th>
              <th>Last Heartbeat</th>
              <th>Equity</th>
              <th>Trades</th>
              <th>Risk Rejects</th>
              <th>Exposure</th>
              <th>Controls</th>
            </tr>
          </thead>
          <tbody>
            {(summary?.teams ?? []).map((team) => (
              <tr key={team.team_id}>
                <td>
                  <strong>{team.team_name}</strong>
                  <br />
                  <span className="muted">{team.team_slug}</span>
                </td>
                <td>
                  <span className={`status ${team.status}`}>{team.is_active ? team.status : "paused"}</span>
                </td>
                <td>{formatDate(team.last_heartbeat)}</td>
                <td>{formatMoney(team.equity_cents)}</td>
                <td>{team.trade_count}</td>
                <td>{team.risk_rejection_count}</td>
                <td>{formatMoney(team.gross_exposure_cents)}</td>
                <td>
                  <div className="actions tight">
                    {team.is_active ? (
                      <button type="button" onClick={() => void teamAction(team.team_id, "pause")}>
                        Pause
                      </button>
                    ) : (
                      <button type="button" onClick={() => void teamAction(team.team_id, "resume")}>
                        Resume
                      </button>
                    )}
                    <button type="button" onClick={() => void teamAction(team.team_id, "reset")}>
                      Reset
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </div>
  );
}
