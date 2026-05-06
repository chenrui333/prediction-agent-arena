import { AdminControls } from "@/components/AdminControls";

export default function AdminPage() {
  return (
    <div className="stack">
      <section className="page-head">
        <div>
          <h1>Admin</h1>
          <p className="muted">Local operator controls for teams, rounds, and exports.</p>
        </div>
      </section>
      <AdminControls />
    </div>
  );
}
