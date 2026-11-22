import { NAT_LIMIT, rateLimitStep, type RateBucket } from "@openintern/proven";

const buckets = new Map<string, RateBucket>();

const DEFAULT_LIMIT_PER_MINUTE = 60;

function envLimit(): number {
  const n = Number(process.env.API_RATE_LIMIT_PER_MINUTE ?? DEFAULT_LIMIT_PER_MINUTE);
  return Number.isInteger(n) && n >= 0 && n < NAT_LIMIT ? n : DEFAULT_LIMIT_PER_MINUTE;
}

/** Fixed-window limiter; each decision comes from the Bend-proven `rate_limit.step`. */
export function rateLimit(
  key: string,
  limit = envLimit(),
  windowMs = 60_000,
): { ok: boolean; remaining: number; resetAt: number } {
  const now = Date.now();
  try {
    const verdict = rateLimitStep(buckets.get(key), now, limit, windowMs);
    buckets.set(key, verdict.bucket);
    return { ok: verdict.ok, remaining: verdict.remaining, resetAt: verdict.bucket.resetAt };
  } catch (err) {
    // Bad limiter input is our bug, not the client's: log it and let the request through.
    console.error("rateLimit:", err instanceof Error ? err.message : err);
    return { ok: true, remaining: 0, resetAt: now + windowMs };
  }
}

export function clientIp(request: Request): string {
  const xf = request.headers.get("x-forwarded-for");
  if (xf) return xf.split(",")[0]?.trim() || "unknown";
  return request.headers.get("x-real-ip") ?? "unknown";
}
