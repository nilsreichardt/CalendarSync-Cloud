import "./globals.css";
import Link from "next/link";
import { auth, signIn, signOut } from "@/auth";

export default async function RootLayout({ children }: { children: React.ReactNode }) {
  const session = await auth();

  return (
    <html lang="en">
      <body>
        <div className="shell">
          <div className="topbar">
            <div>
              <h1 style={{ margin: 0 }}>CalendarSync Cloud</h1>
              <div className="muted">Google account bundles and full sync-rule parity</div>
            </div>
            <div>
              {session ? (
                <form
                  action={async () => {
                    "use server";
                    await signOut();
                  }}
                >
                  <button className="btn secondary">Sign out</button>
                </form>
              ) : (
                <form
                  action={async () => {
                    "use server";
                    await signIn("google");
                  }}
                >
                  <button className="btn">Sign in with Google</button>
                </form>
              )}
            </div>
          </div>
          {session && (
            <div className="tabs">
              <Link className="tab" href="/">
                Connections
              </Link>
              <Link className="tab" href="/rules">
                Rules
              </Link>
              <Link className="tab" href="/runs">
                Runs
              </Link>
            </div>
          )}
          <div style={{ marginTop: 18 }}>{children}</div>
        </div>
      </body>
    </html>
  );
}
