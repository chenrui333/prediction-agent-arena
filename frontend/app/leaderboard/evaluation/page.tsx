import { EvaluationLeaderboard } from "@/components/EvaluationLeaderboard";
import { apiBase, getLeaderboard } from "@/lib/api";
import type { LeaderboardResponse } from "@/lib/types";

export const dynamic = "force-dynamic";

export default async function EvaluationLeaderboardPage() {
  let initial: LeaderboardResponse;
  let initialError = "";
  try {
    initial = await getLeaderboard();
  } catch {
    initial = offlineLeaderboard();
    initialError = "Backend API unavailable. Retrying on refresh.";
  }
  return <EvaluationLeaderboard initial={initial} apiBase={apiBase} initialError={initialError} />;
}

function offlineLeaderboard(): LeaderboardResponse {
  return {
    round: {
      id: 0,
      slug: "offline",
      name: "Evaluation",
      mode: "live_paper",
      status: "paused",
      require_locked_agents: false,
      initial_balance_cents: 0,
    },
    rows: [],
  };
}
