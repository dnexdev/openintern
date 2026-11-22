/**
 * The one way ingest talks to job boards: an honest user-agent, bounded
 * retries that respect Retry-After, and typed failures so a refusal is never
 * mistaken for an empty board.
 *
 * Requests identify as OpenIntern. Callers cannot set User-Agent, Referer,
 * Origin or Cookie: pretending to be a browser or another site is not allowed.
 */

export const OPENINTERN_UA = "OpenIntern/0.1 (+https://github.com/dnexdev/openintern)";

/**
 * refused: 401/403, the site said no. throttled: 429 after retries.
 * not-found: 404/410 (usually a wrong board token). http: any other non-2xx.
 * network: DNS, TLS, reset, timeout. invalid: a 2xx body we could not read.
 * manual-expired: a committed manual capture is too old to publish.
 */
export type FetchFailureKind =
  | "refused"
  | "throttled"
  | "not-found"
  | "http"
  | "network"
  | "invalid"
  | "manual-expired";

export class FetchFailure extends Error {
  constructor(
    readonly kind: FetchFailureKind,
    message: string,
    readonly status?: number,
  ) {
    super(`${kind}: ${message}`);
    this.name = "FetchFailure";
  }
}

const FORBIDDEN_HEADERS = new Set(["user-agent", "referer", "origin", "cookie"]);

export type HttpOptions = {
  method?: "GET" | "POST";
  /** Sent as JSON when an object, as-is when a string. */
  body?: unknown;
  headers?: Record<string, string>;
  /** Total attempts, including the first. */
  attempts?: number;
  timeoutMs?: number;
};

const MAX_RETRY_AFTER_MS = 30_000;

function sleep(ms: number) {
  return new Promise((r) => setTimeout(r, ms));
}

/** Retry-After as milliseconds (seconds or an HTTP date), capped; null if absent or bad. */
export function retryAfterMs(value: string | null, now = Date.now()): number | null {
  if (!value) return null;
  const trimmed = value.trim();
  if (/^\d+$/.test(trimmed)) return Math.min(Number(trimmed) * 1000, MAX_RETRY_AFTER_MS);
  const at = Date.parse(trimmed);
  if (Number.isNaN(at)) return null;
  return Math.min(Math.max(0, at - now), MAX_RETRY_AFTER_MS);
}

function failureFor(status: number, url: string): FetchFailure {
  const msg = `HTTP ${status} for ${url}`;
  if (status === 401 || status === 403) return new FetchFailure("refused", msg, status);
  if (status === 429) return new FetchFailure("throttled", msg, status);
  if (status === 404 || status === 410) return new FetchFailure("not-found", msg, status);
  return new FetchFailure("http", msg, status);
}

/** A 2xx response, or a FetchFailure. Retries 429, 5xx and network errors. */
export async function httpRequest(url: string, opts: HttpOptions = {}): Promise<Response> {
  const headers: Record<string, string> = {};
  for (const [k, v] of Object.entries(opts.headers ?? {})) {
    if (FORBIDDEN_HEADERS.has(k.toLowerCase())) {
      throw new Error(`ingest may not set the ${k} header (honest requests only)`);
    }
    headers[k] = v;
  }
  headers["User-Agent"] = OPENINTERN_UA;
  let body: string | undefined;
  if (opts.body !== undefined) {
    body = typeof opts.body === "string" ? opts.body : JSON.stringify(opts.body);
    headers["Content-Type"] ??= "application/json";
  }

  const attempts = Math.max(1, opts.attempts ?? 3);
  let last: FetchFailure | null = null;
  for (let attempt = 0; attempt < attempts; attempt++) {
    let res: Response;
    try {
      res = await fetch(url, {
        method: opts.method ?? (body === undefined ? "GET" : "POST"),
        headers,
        body,
        redirect: "follow",
        signal: AbortSignal.timeout(opts.timeoutMs ?? 30_000),
      });
    } catch (err) {
      last = new FetchFailure("network", `${err instanceof Error ? err.message : String(err)} for ${url}`);
      await sleep(400 * 2 ** attempt);
      continue;
    }
    if (res.ok) return res;
    last = failureFor(res.status, url);
    if (res.status !== 429 && res.status < 500) throw last;
    await res.body?.cancel();
    await sleep(retryAfterMs(res.headers.get("retry-after")) ?? 400 * 2 ** attempt);
  }
  throw last ?? new FetchFailure("network", `no response for ${url}`);
}

export async function httpJson<T>(url: string, opts: HttpOptions = {}): Promise<T> {
  const res = await httpRequest(url, {
    ...opts,
    headers: { Accept: "application/json", ...opts.headers },
  });
  try {
    return (await res.json()) as T;
  } catch (err) {
    throw new FetchFailure("invalid", `unreadable JSON from ${url}: ${err instanceof Error ? err.message : err}`);
  }
}

export async function httpText(url: string, opts: HttpOptions = {}): Promise<string> {
  const res = await httpRequest(url, {
    ...opts,
    headers: { Accept: "text/html,application/xhtml+xml", ...opts.headers },
  });
  return res.text();
}
