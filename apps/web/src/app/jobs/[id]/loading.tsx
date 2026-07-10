export default function JobLoading() {
  return (
    <div style={{ paddingTop: "1.25rem" }}>
      <div className="skeleton-line w-40" style={{ marginBottom: "1rem" }} />
      <div className="panel">
        <div className="skeleton-card" style={{ border: "none", padding: 0, boxShadow: "none" }}>
          <div className="skeleton-avatar" />
          <div className="skeleton-lines">
            <div className="skeleton-line w-80" />
            <div className="skeleton-line w-60" />
            <div className="skeleton-line w-40" />
            <div className="skeleton-line w-90" />
          </div>
        </div>
      </div>
    </div>
  );
}
