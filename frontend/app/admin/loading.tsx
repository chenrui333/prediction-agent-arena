export default function AdminLoading() {
  return (
    <div className="stack">
      <section className="page-head">
        <div>
          <h1>Admin</h1>
          <p className="muted">Loading operator console.</p>
        </div>
      </section>
      <div className="panel skeleton" />
    </div>
  );
}
