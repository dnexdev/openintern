export default function Loading() {
  return (
    <section className="hero hero-viewport" aria-busy="true">
      <div className="hero-viewport-inner">
        <div className="hero-copy" style={{ width: "100%", maxWidth: "36rem" }}>
          <div className="skeleton-line w-80" style={{ height: "2rem", margin: "0 auto" }} />
          <div
            className="skeleton-lines"
            style={{ marginTop: "1rem", width: "100%" }}
          >
            <div className="skeleton-line w-90" />
            <div className="skeleton-line w-60" />
          </div>
        </div>
      </div>
    </section>
  );
}
