import type { HealthSnapshot } from "@openintern/db";
import { dumpUrl } from "@/lib/job-dump";

const globalForHealth = globalThis as unknown as {
  __openinternHealthSnapshot?: { snapshot: HealthSnapshot; fetchedAt: number };
};

const CACHE_MS = 60 * 60 * 1000;

/** health.json is published next to jobs.json by the dump job. */
export function healthSnapshotUrl() {
  return (
    process.env.HEALTH_SNAPSHOT_URL?.trim() ||
    new URL("health.json", dumpUrl()).toString()
  );
}

/**
 * The pipeline snapshot from the latest dump run, or null if none has been
 * published yet. Never touches the database.
 */
export async function loadHealthSnapshot(): Promise<HealthSnapshot | null> {
  const cached = globalForHealth.__openinternHealthSnapshot;
  if (cached && Date.now() - cached.fetchedAt < CACHE_MS) {
    return cached.snapshot;
  }
  try {
    const res = await fetch(healthSnapshotUrl(), { next: { revalidate: 3600 } });
    if (!res.ok) return null;
    const snapshot = (await res.json()) as HealthSnapshot;
    if (!snapshot?.stats || !Array.isArray(snapshot.recent_ingest_runs)) return null;
    globalForHealth.__openinternHealthSnapshot = { snapshot, fetchedAt: Date.now() };
    return snapshot;
  } catch (err) {
    console.warn("[health] snapshot unavailable:", err);
    return null;
  }
}
