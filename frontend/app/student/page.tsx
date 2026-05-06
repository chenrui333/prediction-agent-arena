import { StudentLaunchpad } from "@/components/StudentLaunchpad";

export default function StudentPage() {
  return (
    <div className="stack">
      <section className="page-head">
        <div>
          <h1>Student Launchpad</h1>
          <p className="muted">Verify your registered agent token and copy the local commands for your laptop agent loop.</p>
        </div>
      </section>
      <StudentLaunchpad />
    </div>
  );
}
