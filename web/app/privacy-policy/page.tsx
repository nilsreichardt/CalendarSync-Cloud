export default function PrivacyPolicyPage() {
  return (
    <div className="card legal-page">
      <span className="legal-page__eyebrow">Legal</span>
      <h2>Privacy Policy</h2>
      <p className="muted">Last updated: March 9, 2026</p>

      <section>
        <h3>What CalendarSync Cloud stores</h3>
        <p>
          CalendarSync Cloud stores the account details, connection metadata, sync rules, and access tokens required to
          link your Google accounts and run calendar synchronization on your behalf.
        </p>
      </section>

      <section>
        <h3>How data is used</h3>
        <p>
          Your data is used only to authenticate you, manage linked accounts, execute the sync rules you configure, and
          show run history inside the application.
        </p>
      </section>

      <section>
        <h3>What calendar content is processed</h3>
        <p>
          Event data may be read, transformed, and written between calendars strictly to perform the synchronization you
          requested. Calendar content is not sold or used for advertising.
        </p>
      </section>

      <section>
        <h3>Sharing</h3>
        <p>
          CalendarSync Cloud does not share your personal information with third parties except when required to operate
          the service, comply with law, or protect the service from abuse.
        </p>
      </section>

      <section>
        <h3>Retention and deletion</h3>
        <p>
          Account connections, sync rules, and related service records are retained while your account is active. You
          can remove linked accounts from the app, and related access tokens will no longer be used for future syncs.
        </p>
      </section>

      <section>
        <h3>Contact</h3>
        <p>
          If you have privacy questions or need data removed, contact the operator of this CalendarSync Cloud
          deployment through the support channel provided with the service.
        </p>
      </section>
    </div>
  );
}
