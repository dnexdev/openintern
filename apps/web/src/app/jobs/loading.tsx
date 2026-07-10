export default function Loading() {
  return (
    <div className="board board-page" style={{ paddingTop: "1rem" }}>
      <aside className="filters-bar">
        <div className="skeleton-line w-40" />
        <div className="skeleton-lines" style={{ marginTop: "0.75rem" }}>
          <div className="skeleton-line w-80" />
          <div className="skeleton-line w-60" />
        </div>
      </aside>
      <section className="job-grid">
        {Array.from({ length: 6 }).map((_, i) => (
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
