import Link from "next/link";
import { auth, signIn } from "@/auth";
import { apiDelete, apiGet, apiPost } from "@/lib/api";
import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";

export default async function ConnectionsPage() {
  const session = await auth();
  if (!session) {
    return (
      <div className="landing-page">
        <section className="landing-hero">
          <div className="landing-hero__copy">
            <div className="landing-eyebrow">Calendar orchestration for real life</div>
            <h2>Keep every Google calendar aligned without managing duplicate events by hand.</h2>
            <p className="landing-lead">
              CalendarSync Cloud lets you connect personal, work, and school accounts, then build precise sync rules
              with filters, cleanup runs, and transformation controls.
            </p>
            <div className="landing-actions">
              <form
                action={async () => {
                  "use server";
                  await signIn("google");
                }}
              >
                <button className="btn landing-btn">Start with Google</button>
              </form>
            </div>
            <div className="landing-metrics">
              <div className="metric-card">
                <strong>Multi-account</strong>
                <span>Bundle separate Google identities into one workspace.</span>
              </div>
              <div className="metric-card">
                <strong>Rule-based</strong>
                <span>Filter, transform, and target exactly the calendars you want.</span>
              </div>
              <div className="metric-card">
                <strong>Traceable</strong>
                <span>Inspect sync runs and clean up mirrored events safely.</span>
              </div>
            </div>
          </div>

          <div className="landing-hero__visual">
            <div className="sync-orbit">
              <div className="sync-orbit__ring sync-orbit__ring--outer" />
              <div className="sync-orbit__ring sync-orbit__ring--inner" />
              <div className="sync-orbit__core">
                <span>CalendarSync</span>
                <strong>Cloud</strong>
              </div>
              <div className="sync-node sync-node--work">Work</div>
              <div className="sync-node sync-node--life">Personal</div>
              <div className="sync-node sync-node--school">School</div>
              <div className="sync-node sync-node--target">Shared plan</div>
            </div>
          </div>
        </section>

        <section className="landing-strip">
          <div className="landing-strip__item">
            <span className="landing-strip__label">Link</span>
            <p>Add every Google account that owns or receives calendar data.</p>
          </div>
          <div className="landing-strip__item">
            <span className="landing-strip__label">Shape</span>
            <p>Apply attendee, title, description, reminder, and timeframe controls.</p>
          </div>
          <div className="landing-strip__item">
            <span className="landing-strip__label">Run</span>
            <p>Track every sync execution and trigger cleanup when a rule changes.</p>
          </div>
        </section>

        <section className="landing-grid">
          <article className="landing-panel landing-panel--accent">
            <div className="landing-panel__eyebrow">Why teams use it</div>
            <h3>One rule system across fragmented calendars.</h3>
            <p>
              Stop forwarding invites manually between accounts. Create stable flows from source calendars into target
              calendars with behavior that stays predictable as events change.
            </p>
          </article>
          <article className="landing-panel">
            <div className="landing-panel__eyebrow">What you control</div>
            <ul className="landing-list">
              <li>Source and target account selection</li>
              <li>Calendar-level routing</li>
              <li>Filters for time range, declined events, and all-day handling</li>
              <li>Transformations for title, description, location, reminders, and links</li>
            </ul>
          </article>
        </section>
      </div>
    );
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
