import { FinalsLeaderboard } from "@/components/FinalsLeaderboard";
import { apiBase, getLeaderboard } from "@/lib/api";

export const dynamic = "force-dynamic";

export default async function FinalsLeaderboardPage() {
  const initial = await getLeaderboard();
  return <FinalsLeaderboard initial={initial} apiBase={apiBase} />;
}
