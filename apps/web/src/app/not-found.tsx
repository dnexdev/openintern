import Link from "next/link";

export default function NotFound() {
  return (
    <section className="hero" style={{ maxWidth: 560 }}>
      <p className="eyebrow">404</p>
      <h1>Page not found</h1>
      <p>
        That URL isn’t in the corpus. Head back to the board, or check the API
        docs if you were looking for an endpoint.
      </p>
      <div className="hero-actions">
        <Link className="btn btn-primary" href="/">
          Browse jobs
        </Link>
        <Link className="btn" href="/docs">
          API docs
        </Link>
      </div>
    </section>
  );
}
