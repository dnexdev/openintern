/** Latest instant the proven freshness code accepts (2^48 ms, year 10889). */
const MAX_MS = 2 ** 48;

function inRange(d: Date): Date | null {
  const t = d.getTime();
  return Number.isFinite(t) && t >= 0 && t < MAX_MS ? d : null;
}

/** Coerce ATS timestamps to a valid Date in [1970, 2^48 ms) or null (never Invalid Date). */
export function safeDate(value: unknown): Date | null {
  if (value == null || value === "") return null;
  if (value instanceof Date) return inRange(value);
  if (typeof value === "number") {
    if (!Number.isFinite(value)) return null;
    // Heuristic: seconds vs milliseconds
    const ms = value < 1e12 ? value * 1000 : value;
    return inRange(new Date(ms));
  }
  if (typeof value === "string") return inRange(new Date(value));
  return null;
}
