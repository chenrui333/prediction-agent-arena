export default function LeaderboardLoading() {
  return (
    <div className="stack">
      <section className="page-head">
        <div>
          <h1>Leaderboard</h1>
          <p className="muted">Loading scores.</p>
        </div>
      </section>
      <div className="table-wrap skeleton" />
    </div>
  );
}
