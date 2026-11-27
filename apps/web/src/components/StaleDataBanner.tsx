import { formatIngestAge } from "@/lib/board";

/**
 * Shown only when the dump is past the 30h freshness window (callers gate with
 * `isDumpStale`). Dump is the default board source, so this is a pipeline stall
 * warning — not a database-offline message.
 */
export function StaleDataBanner({ generatedAt }: { generatedAt: string }) {
  const age = formatIngestAge(generatedAt);
  return (
    <div className="panel stale-banner" role="status">
      <p>
        Daily dump is older than 30 hours
        {age ? (
          <>
            {" "}
            (last refresh <strong>{age}</strong>)
          </>
        ) : null}
        . Ingest or dump may have stalled; listings can be out of date until the
        next successful publish.
      </p>
    </div>
  );
}
