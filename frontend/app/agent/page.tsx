import { AgentLaunchpad } from "@/components/AgentLaunchpad";

export default function AgentPage() {
  return (
    <div className="stack">
      <section className="page-head">
        <div>
          <h1>Agent Launchpad</h1>
          <p className="muted">Verify your registered agent token and copy the local commands for your laptop agent loop.</p>
        </div>
      </section>
      <AgentLaunchpad />
    </div>
  );
}
