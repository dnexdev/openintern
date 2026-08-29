import { formatIngestAge } from "@/lib/board";

export function StaleDataBanner({ generatedAt }: { generatedAt: string }) {
  const age = formatIngestAge(generatedAt);
  return (
    <div className="panel stale-banner" role="status">
      <p>
        Database is temporarily offline — showing cached listings
        {age ? (
          <>
            {" "}
            from the daily dump (<strong>{age}</strong>)
          </>
        ) : (
          " from the latest daily dump"
        )}
        . Listings may be slightly stale until ingest resumes.
      </p>
    </div>
  );
}
