export default function PrivacyPage() {
  return (
    <article className="prose-page">
      <h1>Privacy</h1>
      <p>
        OpenIntern is a no-account product. We do not run user accounts, login,
        or server-side application tracking.
      </p>
      <h2>What we store</h2>
      <ul>
        <li>
          <strong>Applied marks</strong> live only in your browser{" "}
          <code className="mono">localStorage</code>. Clearing site data removes
          them. We never see them.
        </li>
        <li>
          <strong>Job corpus</strong> is public internship metadata polled from
          employer ATS APIs (titles, locations, apply URLs, etc.).
        </li>
        <li>
          <strong>API / hosting logs</strong> may include standard request
          metadata (IP, user-agent, path) from Vercel or similar for rate
          limiting and abuse prevention.
        </li>
        <li>
          <strong>Aggregate analytics</strong> (optional): when enabled, we use{" "}
          <a href="https://plausible.io">Plausible</a> for privacy-friendly page
          view counts. No cookies, no accounts, no personal profiles.
        </li>
      </ul>
      <h2>What we don’t do</h2>
      <ul>
        <li>No accounts, cookies for identity, or email collection on the board.</li>
        <li>Apply links go to the employer — we don’t proxy applications.</li>
        <li>We don’t sell listings or personal data.</li>
      </ul>
      <h2>Contact</h2>
      <p>
        Questions: open an issue on{" "}
        <a href="https://github.com/dnexdev/openintern">github.com/dnexdev/openintern</a>.
      </p>
    </article>
  );
}
