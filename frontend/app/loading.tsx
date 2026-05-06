export default function Loading() {
  return (
    <div className="stack">
      <section className="page-head">
        <div>
          <h1>Loading</h1>
          <p className="muted">Connecting to the arena backend.</p>
        </div>
      </section>
      <div className="panel skeleton" />
    </div>
  );
}
