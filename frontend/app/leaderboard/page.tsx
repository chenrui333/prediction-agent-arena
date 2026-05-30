import { LeaderboardTable } from "@/components/LeaderboardTable";
import { formatAPIError, getLeaderboard, isNoActiveRoundError } from "@/lib/api";

export const dynamic = "force-dynamic";

export default async function LeaderboardPage() {
	const result = await getLeaderboard()
		.then((initial) => ({ initial, error: null }))
		.catch((error: unknown) => ({ initial: null, error }));
	if (result.error) {
		if (isNoActiveRoundError(result.error)) {
			return (
				<div className="stack">
					<section className="page-head">
						<div>
							<h1>Leaderboard</h1>
							<p className="muted">No active round is currently running.</p>
						</div>
					</section>
					<div className="notice">Agents need an active round before teams can appear on the leaderboard.</div>
				</div>
			);
		}
		return (
			<div className="stack">
				<section className="page-head">
					<div>
						<h1>Leaderboard</h1>
						<p className="muted">The leaderboard could not be loaded.</p>
					</div>
				</section>
				<div className="notice error">Backend API unavailable: {formatAPIError(result.error)}</div>
			</div>
		);
	}
	if (!result.initial) {
		return (
			<div className="stack">
				<section className="page-head">
					<div>
						<h1>Leaderboard</h1>
						<p className="muted">The leaderboard could not be loaded.</p>
					</div>
				</section>
				<div className="notice error">Unexpected empty leaderboard response.</div>
			</div>
		);
	}
	const initial = result.initial;
	return <LeaderboardTable initial={initial} />;
}
