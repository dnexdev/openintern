export default function Loading() {
  return (
    <div className="layout" style={{ paddingTop: "1rem" }}>
      <aside className="sidebar">
        <div className="skeleton-line w-40" />
        <div className="skeleton-lines" style={{ marginTop: "1rem" }}>
          <div className="skeleton-line w-80" />
          <div className="skeleton-line w-60" />
          <div className="skeleton-line w-90" />
          <div className="skeleton-line w-40" />
        </div>
      </aside>
      <section className="job-list">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="skeleton-card">
            <div className="skeleton-avatar" />
            <div className="skeleton-lines">
              <div className="skeleton-line w-80" />
              <div className="skeleton-line w-60" />
              <div className="skeleton-line w-40" />
            </div>
          </div>
        ))}
      </section>
    </div>
  );
}
