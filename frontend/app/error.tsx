"use client";

export default function ErrorPage({ error, reset }: { error: Error; reset: () => void }) {
  return (
    <div className="stack">
      <section className="page-head">
        <div>
          <h1>Frontend Error</h1>
          <p className="muted">The page could not load from the local arena backend.</p>
        </div>
        <button type="button" className="primary" onClick={reset}>
          Retry
        </button>
      </section>
      <div className="notice error">{error.message}</div>
    </div>
  );
}
