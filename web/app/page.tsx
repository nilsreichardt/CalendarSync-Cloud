import { auth } from "@/auth";
import { apiDelete, apiGet, apiPost } from "@/lib/api";
import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";

export default async function ConnectionsPage() {
  const session = await auth();
  if (!session) {
    return <div className="card">Sign in to manage linked Google accounts.</div>;
  }

  const data = await apiGet("/api/connections");
  const connections = data.connections ?? [];

  return (
    <div>
      <div className="card">
        <h3 style={{ marginTop: 0 }}>Linked Accounts</h3>
        <p className="muted">Bundle personal, work and school Google accounts under one CalendarSync user.</p>
        <form
          action={async () => {
            "use server";
            const payload = await apiPost("/api/connections/google/start", {});
            redirect(payload.url);
          }}
        >
          <button className="btn">Link Google account</button>
        </form>
      </div>

      {connections.length === 0 && <div className="card muted">No linked accounts yet.</div>}
      {connections.map((connection: any) => (
        <div key={connection.id} className="card">
          <div style={{ display: "flex", justifyContent: "space-between" }}>
            <div>
              <strong>{connection.email}</strong>
              <div className="muted">{connection.displayName || "Google account"}</div>
            </div>
            <form
              action={async () => {
                "use server";
                await apiDelete(`/api/connections/${connection.id}`);
                revalidatePath("/");
              }}
            >
              <button className="btn secondary">Remove</button>
            </form>
          </div>
        </div>
      ))}
    </div>
  );
}
