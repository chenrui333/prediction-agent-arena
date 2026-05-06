import Link from "next/link";

export default function NotFound() {
  return (
    <div className="stack">
      <section className="page-head">
        <div>
          <h1>Not Found</h1>
          <p className="muted">That arena page or team is not available for the active round.</p>
        </div>
        <Link className="button primary" href="/leaderboard">
          Back to leaderboard
        </Link>
      </section>
    </div>
  );
}
