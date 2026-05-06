import { LeaderboardTable } from "@/components/LeaderboardTable";
import { apiBase, getLeaderboard } from "@/lib/api";

export const dynamic = "force-dynamic";

export default async function LeaderboardPage() {
  const initial = await getLeaderboard();
  return <LeaderboardTable initial={initial} apiBase={apiBase} />;
}
