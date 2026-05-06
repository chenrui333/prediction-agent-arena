"use client";

import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  apiBase,
  formatAPIError,
  formatDateTime,
  formatMoney,
  getAdminHealth,
  getAdminMarkets,
  getAdminSummary,
  getRoundAgents,
  getRounds,
  getRoundTeams,
  getTeamAgents,
} from "@/lib/api";
import type { AdminSummary, Agent, ArenaHealth, Market, Round, RoundAgent, RoundTeam } from "@/lib/types";

type Message = { type: "ok" | "error"; text: string };
const adminTokenStorageKey = "prediction-agent-arena.admin-token";

export function AdminControls() {
  const [token, setToken] = useState(() => {
    if (typeof window === "undefined") {
      return "";
    }
    return window.localStorage.getItem(adminTokenStorageKey) ?? "";
  });
  const [message, setMessage] = useState<Message | null>(null);
  const [summary, setSummary] = useState<AdminSummary | null>(null);
  const [health, setHealth] = useState<ArenaHealth | null>(null);
  const [rounds, setRounds] = useState<Round[]>([]);
  const [adminMarkets, setAdminMarkets] = useState<Market[]>([]);
  const [roundTeams, setRoundTeams] = useState<RoundTeam[]>([]);
  const [roundAgents, setRoundAgents] = useState<RoundAgent[]>([]);
  const [roundScopeID, setRoundScopeID] = useState("");
  const [roundScopeLoading, setRoundScopeLoading] = useState(false);
  const [agentsByTeam, setAgentsByTeam] = useState<Record<number, Agent[]>>({});
  const [teamSlug, setTeamSlug] = useState("");
  const [teamName, setTeamName] = useState("");
  const [agentTeamID, setAgentTeamID] = useState("");
  const [agentSlug, setAgentSlug] = useState("default");
  const [agentName, setAgentName] = useState("");
  const [roundID, setRoundID] = useState("");
  const [roundSlug, setRoundSlug] = useState("");
  const [roundName, setRoundName] = useState("");
  const [loading, setLoading] = useState(false);
  const roundScopeRequestID = useRef(0);

  const selectedRoundID = useMemo(() => {
    if (roundID) {
      return roundID;
    }
    return String(summary?.active_round?.id ?? summary?.latest_round?.id ?? "");
  }, [roundID, summary]);

  const selectedRound = useMemo(() => {
    const id = Number(selectedRoundID);
    return rounds.find((round) => round.id === id) ?? summary?.active_round ?? summary?.latest_round ?? null;
  }, [rounds, selectedRoundID, summary]);

  const readiness = useMemo(() => {
    const roundScopeReady = !selectedRoundID || roundScopeID === selectedRoundID;
    const scopedRoundTeams = roundScopeReady ? roundTeams : [];
    const scopedRoundAgents = roundScopeReady ? roundAgents : [];
    return buildReadiness(summary, health, selectedRound, adminMarkets, scopedRoundTeams, scopedRoundAgents, agentsByTeam, roundScopeReady);
  }, [adminMarkets, agentsByTeam, health, roundAgents, roundScopeID, roundTeams, selectedRound, selectedRoundID, summary]);

  useEffect(() => {
    if (token) {
      window.localStorage.setItem(adminTokenStorageKey, token);
      return;
    }
    window.localStorage.removeItem(adminTokenStorageKey);
  }, [token]);

  const loadRoundScope = useCallback(
    async (id: string) => {
      const numericID = Number(id);
      if (!token || !Number.isFinite(numericID) || numericID <= 0) {
        setRoundScopeID("");
        setRoundTeams([]);
        setRoundAgents([]);
        return;
      }

      const requestID = roundScopeRequestID.current + 1;
      roundScopeRequestID.current = requestID;
      const scopeID = String(numericID);
      setRoundScopeLoading(true);
      try {
        const [nextRoundTeams, nextRoundAgents] = await Promise.all([getRoundTeams(token, numericID), getRoundAgents(token, numericID)]);
        if (roundScopeRequestID.current !== requestID) {
          return;
        }
        setRoundTeams(nextRoundTeams);
        setRoundAgents(nextRoundAgents);
        setRoundScopeID(scopeID);
      } catch (err) {
        if (roundScopeRequestID.current !== requestID) {
          return;
        }
        setRoundTeams([]);
        setRoundAgents([]);
        setRoundScopeID(scopeID);
        setMessage({ type: "error", text: formatAPIError(err) });
      } finally {
        if (roundScopeRequestID.current === requestID) {
          setRoundScopeLoading(false);
        }
      }
    },
    [token],
  );

  const refresh = useCallback(async () => {
    if (!token) {
      setMessage({ type: "error", text: "admin token is required" });
      return;
    }
    setLoading(true);
    setMessage(null);
    try {
      const [nextSummary, nextRounds, nextHealth, nextMarkets] = await Promise.all([
        getAdminSummary(token),
        getRounds(token),
        getAdminHealth(token),
        getAdminMarkets(token),
      ]);
      const inferredID = Number(roundID || nextSummary.active_round?.id || nextSummary.latest_round?.id || 0);
      const agentEntries = await Promise.all(
        nextSummary.teams.map(async (team) => {
          const agents = await getTeamAgents(token, team.team_id);
          return [team.team_id, agents] as const;
        }),
      );
      setSummary(nextSummary);
      setRounds(nextRounds);
      setHealth(nextHealth);
      setAdminMarkets(nextMarkets);
      setAgentsByTeam(Object.fromEntries(agentEntries));
      if (!roundID) {
        if (inferredID) {
          setRoundID(String(inferredID));
        }
      }
      await loadRoundScope(String(inferredID));
    } catch (err) {
      setMessage({ type: "error", text: formatAPIError(err) });
    } finally {
      setLoading(false);
    }
  }, [loadRoundScope, roundID, token]);

  useEffect(() => {
    if (!token) {
      return;
    }
    const timer = window.setInterval(() => {
      void refresh();
    }, 7000);
    return () => window.clearInterval(timer);
  }, [refresh, token]);

  const selectRound = useCallback(
    (id: string) => {
      setRoundID(id);
      void loadRoundScope(id);
    },
    [loadRoundScope],
  );

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
      const fallback = text || `HTTP ${response.status}`;
      let message = fallback;
      try {
        const parsed = JSON.parse(text) as { error?: { code?: string; message?: string } };
        if (parsed.error?.message) {
          const code = parsed.error.code;
          message = code ? `${code}: ${parsed.error.message}` : parsed.error.message;
        }
      } catch {
        message = fallback;
      }
      throw new Error(message);
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

  async function createAgent(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!agentTeamID) {
      setMessage({ type: "error", text: "team is required" });
      return;
    }
    try {
      const text = await request(`/api/v1/admin/teams/${agentTeamID}/agents`, {
        method: "POST",
        body: JSON.stringify({ slug: agentSlug, name: agentName || agentSlug, kind: "student" }),
      });
      setMessage({ type: "ok", text });
      setAgentSlug("default");
      setAgentName("");
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "create agent failed" });
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

  async function roundAction(
    action: "activate" | "pause" | "complete" | "reset" | "settle" | "freeze-leaderboard" | "require-locked-agents" | "allow-unlocked-agents",
  ) {
    if (!selectedRoundID) {
      setMessage({ type: "error", text: "round id is required" });
      return;
    }
    try {
      const body = action === "settle" ? JSON.stringify({ confirm: "settle_active_round" }) : undefined;
      const text = await request(`/api/v1/admin/rounds/${selectedRoundID}/${action}`, { method: "POST", body });
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

  async function compactSnapshots() {
    if (!selectedRoundID) {
      setMessage({ type: "error", text: "round id is required" });
      return;
    }
    try {
      const text = await request("/api/v1/admin/snapshots/compact", {
        method: "POST",
        body: JSON.stringify({ round_id: Number(selectedRoundID), keep_every: "5m" }),
      });
      setMessage({ type: "ok", text });
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "compact snapshots failed" });
    }
  }

  async function compactAudit() {
    try {
      const text = await request("/api/v1/admin/audit/compact", {
        method: "POST",
        body: JSON.stringify({ older_than: "14d" }),
      });
      setMessage({ type: "ok", text });
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "compact audit failed" });
    }
  }

  async function teamAction(teamID: number, action: "pause" | "resume" | "reset" | "rotate-token") {
    if (action === "reset" && !selectedRoundID) {
      setMessage({ type: "error", text: "round id is required for team reset" });
      return;
    }
    try {
      const path =
        action === "reset"
          ? `/api/v1/admin/rounds/${selectedRoundID}/teams/${teamID}/reset`
          : `/api/v1/admin/teams/${teamID}/${action}`;
      const text = await request(path, { method: "POST" });
      setMessage({ type: "ok", text });
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : `${action} failed` });
    }
  }

  async function roundTeamAction(teamID: number, action: "enroll" | "pause" | "resume" | "withdraw") {
    if (!selectedRoundID) {
      setMessage({ type: "error", text: "round id is required" });
      return;
    }
    try {
      const text = await request(`/api/v1/admin/rounds/${selectedRoundID}/teams/${teamID}/${action}`, { method: "POST" });
      setMessage({ type: "ok", text });
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : `${action} failed` });
    }
  }

  async function agentAction(agentID: number, action: "pause" | "resume" | "revoke" | "rotate-token") {
    try {
      const text = await request(`/api/v1/admin/agents/${agentID}/${action}`, { method: "POST" });
      setMessage({ type: "ok", text });
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : `${action} failed` });
    }
  }

  async function roundActionFor(id: number, action: "activate" | "pause" | "complete") {
    try {
      const text = await request(`/api/v1/admin/rounds/${id}/${action}`, { method: "POST" });
      setMessage({ type: "ok", text });
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : `${action} failed` });
    }
  }

  async function exportRoundFor(id: number) {
    try {
      const text = await request(`/api/v1/admin/export/${id}`, { method: "GET" });
      setMessage({ type: "ok", text });
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "export failed" });
    }
  }

  return (
    <div className="stack">
      <section className="form-band stack">
        <div className="section-head">
          <div>
            <h2>Access</h2>
            <p className="muted">Active round: {summary?.active_round ? `${summary.active_round.name} (${summary.active_round.status})` : "none"}</p>
            {health ? (
              <p className="muted">
                Health: {health.status} / DB {health.db_ok ? "ok" : "down"} / Redis {health.redis_ok ? "ok" : "degraded"} / worker{" "}
                {formatDateTime(health.latest_worker_heartbeat_at)}
              </p>
            ) : null}
          </div>
          <button type="button" className="primary" onClick={() => void refresh()} disabled={loading}>
            {loading ? "Refreshing" : "Refresh"}
          </button>
          {token ? (
            <button type="button" onClick={() => setToken("")}>
              Forget token
            </button>
          ) : null}
        </div>
        <label>
          Admin token
          <input value={token} onChange={(event) => setToken(event.target.value)} type="password" placeholder="dev-admin-token" />
        </label>
        {message ? <pre className={`notice ${message.type === "error" ? "error" : ""}`}>{message.text}</pre> : null}
      </section>

      <section className="form-band stack">
        <div className="section-head">
          <div>
            <h2>Round Readiness</h2>
            <p className="muted">
              {selectedRound ? `${selectedRound.name} (${selectedRound.slug})` : "Select or create a round to run readiness checks."}
              {roundScopeLoading ? " Loading round-scoped data..." : ""}
            </p>
          </div>
          <span className={`status ${readiness.every((item) => item.state === "ok") ? "active" : "paused"}`}>
            {readiness.filter((item) => item.state === "ok").length}/{readiness.length} ready
          </span>
        </div>
        <div className="readiness-grid">
          {readiness.map((item) => (
            <div key={item.label} className={`readiness-item ${item.state}`}>
              <strong>{item.label}</strong>
              <span>{item.detail}</span>
            </div>
          ))}
        </div>
      </section>

      <div className="split">
        <section className="form-band stack">
          <h2>Round Controls</h2>
          <label>
            Round ID
            <select value={roundID || selectedRoundID} onChange={(event) => selectRound(event.target.value)}>
              <option value="">Select round</option>
              {rounds.map((round) => (
                <option key={round.id} value={round.id}>
                  {round.slug} / {round.status}
                </option>
              ))}
            </select>
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
            <button type="button" onClick={() => void roundAction("require-locked-agents")}>
              Require locked agents
            </button>
            <button type="button" onClick={() => void roundAction("allow-unlocked-agents")}>
              Allow unlocked
            </button>
            <button type="button" onClick={() => void roundAction("settle")}>
              Settle
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
            <button type="button" onClick={() => void compactSnapshots()}>
              Compact
            </button>
            <button type="button" onClick={() => void compactAudit()}>
              Compact audit
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

      <section className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Round</th>
              <th>Status</th>
              <th>Mode</th>
              <th>Agent Lock</th>
              <th>Initial Balance</th>
              <th>Updated</th>
              <th>Controls</th>
            </tr>
          </thead>
          <tbody>
            {rounds.map((round) => (
              <tr key={round.id}>
                <td>
                  <strong>{round.name}</strong>
                  <br />
                  <span className="muted">{round.slug}</span>
                </td>
                <td>
                  <span className={`status ${round.status}`}>{round.status}</span>
                </td>
                <td>{round.mode}</td>
                <td>{round.require_locked_agents ? "required" : "open"}</td>
                <td>{formatMoney(round.initial_balance_cents)}</td>
                <td>{formatDateTime(round.updated_at)}</td>
                <td>
                  <div className="actions tight">
                    <button type="button" onClick={() => selectRound(String(round.id))}>
                      Select
                    </button>
                    <button type="button" onClick={() => void roundActionFor(round.id, "activate")}>
                      Activate
                    </button>
                    <button type="button" onClick={() => void roundActionFor(round.id, "pause")}>
                      Pause
                    </button>
                    <button type="button" onClick={() => void roundActionFor(round.id, "complete")}>
                      Complete
                    </button>
                    <button type="button" onClick={() => void exportRoundFor(round.id)}>
                      Export
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {rounds.length === 0 ? (
              <tr>
                <td colSpan={7} className="muted">
                  Enter an admin token and refresh to load rounds.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </section>

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

      <section className="form-band stack">
        <h2>Create Agent</h2>
        <form className="form-grid" onSubmit={createAgent}>
          <label>
            Team
            <select value={agentTeamID} onChange={(event) => setAgentTeamID(event.target.value)} required>
              <option value="">Select team</option>
              {(summary?.teams ?? []).map((team) => (
                <option key={team.team_id} value={team.team_id}>
                  {team.team_slug}
                </option>
              ))}
            </select>
          </label>
          <label>
            Slug
            <input value={agentSlug} onChange={(event) => setAgentSlug(event.target.value)} required />
          </label>
          <label>
            Name
            <input value={agentName} onChange={(event) => setAgentName(event.target.value)} />
          </label>
          <button className="primary" type="submit">
            Create Agent
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
              <th>Agents</th>
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
                <td>{formatDateTime(team.last_heartbeat)}</td>
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
                    <button type="button" onClick={() => void roundTeamAction(team.team_id, "enroll")}>
                      Enroll
                    </button>
                    <button type="button" onClick={() => void roundTeamAction(team.team_id, "pause")}>
                      Round pause
                    </button>
                    <button type="button" onClick={() => void roundTeamAction(team.team_id, "resume")}>
                      Round resume
                    </button>
                    <button type="button" onClick={() => void roundTeamAction(team.team_id, "withdraw")}>
                      Withdraw
                    </button>
                    <button type="button" onClick={() => void teamAction(team.team_id, "rotate-token")}>
                      Rotate token
                    </button>
                  </div>
                </td>
                <td>
                  <div className="stack compact">
                    {(agentsByTeam[team.team_id] ?? []).map((agent) => (
                      <div key={agent.id} className="inline-row">
                        <span>
                          <strong>{agent.slug}</strong> <span className={`status ${agent.status}`}>{agent.status}</span>
                        </span>
                        <span className="actions tight">
                          {agent.status === "active" ? (
                            <button type="button" onClick={() => void agentAction(agent.id, "pause")}>
                              Pause
                            </button>
                          ) : agent.status === "paused" ? (
                            <button type="button" onClick={() => void agentAction(agent.id, "resume")}>
                              Resume
                            </button>
                          ) : null}
                          <button type="button" onClick={() => void agentAction(agent.id, "revoke")}>
                            Revoke
                          </button>
                          <button type="button" onClick={() => void agentAction(agent.id, "rotate-token")}>
                            Rotate
                          </button>
                        </span>
                      </div>
                    ))}
                    {(agentsByTeam[team.team_id] ?? []).length === 0 ? <span className="muted">No registered agents</span> : null}
                  </div>
                </td>
              </tr>
            ))}
            {(summary?.teams ?? []).length === 0 ? (
              <tr>
                <td colSpan={9} className="muted">
                  Enter an admin token and refresh to load teams.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </section>
    </div>
  );
}

type ReadinessItem = {
  label: string;
  state: "ok" | "warn" | "error";
  detail: string;
};

function buildReadiness(
  summary: AdminSummary | null,
  health: ArenaHealth | null,
  round: Round | null,
  markets: Market[],
  roundTeams: RoundTeam[],
  roundAgents: RoundAgent[],
  agentsByTeam: Record<number, Agent[]>,
  roundScopeReady: boolean,
): ReadinessItem[] {
  const activeTeams = summary?.teams.filter((team) => team.is_active) ?? [];
  const activeRoundTeams = roundTeams.filter((team) => team.status === "active" && team.team_is_active);
  const lockedAgentsRequired = Boolean(round?.require_locked_agents || round?.mode === "replay");
  const activeRoundTeamIDs = new Set(activeRoundTeams.map((team) => team.team_id));
  const lockedTeamIDs = new Set(roundAgents.map((agent) => agent.team_id));
  const lockedActiveTeamCount = activeRoundTeams.filter((team) => lockedTeamIDs.has(team.team_id)).length;
  const missingLockedTeamCount = activeRoundTeams.filter((team) => !lockedTeamIDs.has(team.team_id)).length;
  const extraLockedTeamCount = roundAgents.filter((agent) => !activeRoundTeamIDs.has(agent.team_id)).length;
  const invalidLockedTeamCount = roundAgents.filter((lock) => {
    if (!activeRoundTeamIDs.has(lock.team_id)) {
      return false;
    }
    const agent = (agentsByTeam[lock.team_id] ?? []).find((item) => item.id === lock.agent_id);
    return !agent || agent.team_id !== lock.team_id || agent.status !== "active";
  }).length;
  const workerFresh = isFresh(health?.latest_worker_heartbeat_at, 2 * 60 * 1000);
  const openMarkets = markets.filter((market) => market.status === "open" || market.status === "active");

  return [
    {
      label: "Backend",
      state: health?.db_ok ? (health.redis_ok ? "ok" : "warn") : "error",
      detail: health ? `DB ${health.db_ok ? "ok" : "down"}, Redis ${health.redis_ok ? "ok" : "degraded"}` : "health not loaded",
    },
    {
      label: "Worker",
      state: workerFresh ? "ok" : "warn",
      detail: health?.latest_worker_heartbeat_at ? `last heartbeat ${formatDateTime(health.latest_worker_heartbeat_at)}` : "worker heartbeat missing",
    },
    {
      label: "Round",
      state: round ? (round.status === "draft" || round.status === "paused" ? "warn" : "ok") : "error",
      detail: round ? `${round.status} / ${round.mode}` : "no active or latest round",
    },
    {
      label: "Teams",
      state: !roundScopeReady ? "warn" : activeRoundTeams.length > 0 ? "ok" : activeTeams.length > 0 ? "warn" : "error",
      detail: !roundScopeReady ? "loading selected round enrollment" : `${activeRoundTeams.length} active enrolled / ${activeTeams.length} active teams`,
    },
    {
      label: "Markets",
      state: markets.length > 0 ? "ok" : "warn",
      detail: `${markets.length} in catalog / ${openMarkets.length} open`,
    },
    {
      label: "Final Locks",
      state: !lockedAgentsRequired
        ? "ok"
        : !roundScopeReady
          ? "warn"
        : activeRoundTeams.length > 0 && missingLockedTeamCount === 0 && invalidLockedTeamCount === 0
          ? "ok"
          : "error",
      detail: lockedAgentsRequired
        ? !roundScopeReady
          ? "loading selected round locks"
          : `${lockedActiveTeamCount} locked / ${activeRoundTeams.length} active enrolled${
              missingLockedTeamCount > 0 ? `, ${missingLockedTeamCount} missing` : ""
            }${invalidLockedTeamCount > 0 ? `, ${invalidLockedTeamCount} invalid` : ""}${
              extraLockedTeamCount > 0 ? `, ${extraLockedTeamCount} non-active` : ""
            }`
        : "not required",
    },
  ];
}

function isFresh(value: string | undefined, maxAgeMs: number) {
  if (!value) {
    return false;
  }
  const ts = Date.parse(value);
  return Number.isFinite(ts) && Date.now() - ts <= maxAgeMs;
}
