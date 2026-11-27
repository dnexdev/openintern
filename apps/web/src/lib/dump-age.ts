/** Same threshold as Go `board.MaxDumpAge` and `/api/v1/status` (issue #59). */
export const MAX_DUMP_AGE_MS = 30 * 60 * 60 * 1000;

/** True when the dump is older than {@link MAX_DUMP_AGE_MS} (or unparseable). */
export function isDumpStale(generatedAt: string, now = Date.now()): boolean {
  const generated = new Date(generatedAt).getTime();
  if (!Number.isFinite(generated)) return true;
  return now - generated > MAX_DUMP_AGE_MS;
}
