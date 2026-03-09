export default function TermsOfServicePage() {
  return (
    <div className="card legal-page">
      <span className="legal-page__eyebrow">Legal</span>
      <h2>Terms of Service</h2>
      <p className="muted">Last updated: March 9, 2026</p>

      <section>
        <h3>Use of the service</h3>
        <p>
          CalendarSync Cloud lets you connect supported calendar accounts and create synchronization rules. You are
          responsible for the accounts you connect and for reviewing the rules you enable.
        </p>
      </section>

      <section>
        <h3>Acceptable use</h3>
        <p>
          You may not use the service to violate laws, infringe rights, interfere with other users, or attempt
          unauthorized access to the application or its infrastructure.
        </p>
      </section>

      <section>
        <h3>Availability</h3>
        <p>
          The service may change, be updated, or be interrupted at any time. Sync jobs depend on third-party providers,
          so delays or failures can occur outside the operator&apos;s control.
        </p>
      </section>

      <section>
        <h3>Your data and permissions</h3>
        <p>
          By connecting an account, you authorize CalendarSync Cloud to access and update calendar data as needed to
          carry out your configured synchronization rules.
        </p>
      </section>

      <section>
        <h3>Disclaimer</h3>
        <p>
          The service is provided on an as-is basis without warranties of uninterrupted operation, accuracy, or fitness
          for a particular purpose.
        </p>
      </section>

      <section>
        <h3>Liability</h3>
        <p>
          To the maximum extent allowed by law, the operator of this deployment is not liable for indirect, incidental,
          special, or consequential damages arising from use of the service.
        </p>
      </section>

      <section>
        <h3>Changes</h3>
        <p>These terms may be updated from time to time by publishing a revised version on this page.</p>
      </section>
    </div>
  );
}
