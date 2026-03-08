import { auth } from "@/auth";
import { apiGet } from "@/lib/api";

export default async function RunsPage() {
  const session = await auth();
  if (!session) {
    return <div className="card">Sign in to inspect sync runs.</div>;
  }

  const data = await apiGet("/api/runs");
  const runs = data.runs ?? [];

  return (
    <div>
      <div className="card">
        <h3 style={{ marginTop: 0 }}>Run History</h3>
        <div className="muted">Manual and scheduler-triggered runs with status and error output.</div>
      </div>
      {runs.length === 0 && <div className="card muted">No runs yet.</div>}
      {runs.map((run: any) => (
        <div key={run.id} className="card">
          <strong>{run.status}</strong> ({run.triggerType})
          <div className="muted">
            rule {run.ruleId} at {run.createdAt}
          </div>
          {run.error && <pre style={{ whiteSpace: "pre-wrap" }}>{run.error}</pre>}
        </div>
      ))}
    </div>
  );
}
