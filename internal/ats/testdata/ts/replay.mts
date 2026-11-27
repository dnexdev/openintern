// Replays a recorded board (internal/ats/testdata/fixtures/<name>.json.gz)
// through the TypeScript adapter and prints the golden JSON.
//   TZ=UTC tsx replay.mts <packages/ingest/src> <fixture.json.gz>
import fs from "node:fs";
import path from "node:path";
import zlib from "node:zlib";
const src = process.argv[2]!;
const fx = JSON.parse(zlib.gunzipSync(fs.readFileSync(process.argv[3]!)).toString("utf8"));
const byKey = new Map<string, any[]>();
const used = new Map<string, number>();
for (const r of fx.requests) {
  const k = `${r.method} ${r.url} ${r.reqBody ?? ""}`;
  byKey.set(k, [...(byKey.get(k) ?? []), r]);
}
// No pacing or retry waits.
const realSetTimeout = globalThis.setTimeout;
(globalThis as any).setTimeout = (fn: any, _ms?: number, ...a: any[]) => realSetTimeout(fn, 0, ...a);
globalThis.fetch = (async (input: any, init?: any) => {
  const url = typeof input === "string" ? input : input.url ?? String(input);
  const method = (init?.method ?? "GET").toUpperCase();
  const body = init?.body == null ? "" : String(init.body);
  const k = `${method} ${url} ${body}`;
  const q = byKey.get(k) ?? [];
  const n = used.get(k) ?? 0;
  used.set(k, n + 1);
  if (q.length === 0) {
    console.error(`unrecorded request ${k}`);
    throw new TypeError("fetch failed");
  }
  const rec = q[Math.min(n, q.length - 1)];
  if (rec.error) throw new TypeError(rec.error);
  return new Response([101, 204, 205, 304].includes(rec.status) ? null : rec.body, { status: rec.status, headers: rec.headers ?? {} });
}) as typeof fetch;
const { fetchJobsForAts } = await import(path.join(src, "ats.ts"));
const { FetchFailure } = await import(path.join(src, "http.ts"));
const { safeDate } = await import(path.join(src, "safe-date.ts"));
let out: unknown;
try {
  const jobs = await fetchJobsForAts(fx.ats, fx.token);
  out = { jobs: jobs.map((j: any) => ({ ...j, postedAt: safeDate(j.postedAt)?.toISOString() ?? null })) };
} catch (err: any) {
  out = { error: { kind: err instanceof FetchFailure ? err.kind : null, message: err.message } };
}
process.stdout.write(JSON.stringify(out, null, 1) + "\n");
