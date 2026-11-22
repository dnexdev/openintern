import assert from "node:assert/strict";
import http from "node:http";
import type { AddressInfo } from "node:net";
import { FetchFailure, OPENINTERN_UA, httpJson, httpRequest, retryAfterMs } from "./http.js";

const hits = new Map<string, number>();
const seenHeaders: http.IncomingHttpHeaders[] = [];
const server = http.createServer((req, res) => {
  const n = (hits.get(req.url!) ?? 0) + 1;
  hits.set(req.url!, n);
  seenHeaders.push(req.headers);
  const reply = (status: number, body = "{}", headers: Record<string, string> = {}) => {
    res.writeHead(status, { "content-type": "application/json", ...headers });
    res.end(body);
  };
  if (req.url === "/ok") return reply(200, '{"a":1}');
  if (req.url === "/forbidden") return reply(403);
  if (req.url === "/gone") return reply(404);
  if (req.url === "/flaky") return n < 3 ? reply(503) : reply(200, '{"ok":true}');
  if (req.url === "/throttled") return n < 2 ? reply(429, "{}", { "retry-after": "0" }) : reply(200, '{"ok":true}');
  if (req.url === "/always429") return reply(429, "{}", { "retry-after": "0" });
  if (req.url === "/bad-json") return reply(200, "not json");
  reply(500);
});
await new Promise<void>((r) => server.listen(0, "127.0.0.1", r));
const base = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;

const kindOf = async (p: Promise<unknown>) => {
  try {
    await p;
    return "ok";
  } catch (err) {
    return err instanceof FetchFailure ? err.kind : `other: ${err}`;
  }
};

try {
  assert.deepEqual(await httpJson(`${base}/ok`), { a: 1 });
  assert.equal(seenHeaders.at(-1)!["user-agent"], OPENINTERN_UA, "honest user-agent");
  assert.equal(await kindOf(httpRequest(`${base}/forbidden`)), "refused");
  assert.equal(hits.get("/forbidden"), 1, "a refusal is not retried");
  assert.equal(await kindOf(httpRequest(`${base}/gone`)), "not-found");
  assert.deepEqual(await httpJson(`${base}/flaky`), { ok: true }, "5xx is retried");
  assert.deepEqual(await httpJson(`${base}/throttled`), { ok: true }, "429 is retried after Retry-After");
  assert.equal(await kindOf(httpRequest(`${base}/always429`, { attempts: 2 })), "throttled");
  assert.equal(hits.get("/always429"), 2, "bounded attempts");
  assert.equal(await kindOf(httpJson(`${base}/bad-json`)), "invalid");
  assert.equal(await kindOf(httpRequest("http://127.0.0.1:1/", { attempts: 1 })), "network");
  for (const header of ["Referer", "origin", "User-Agent", "Cookie"]) {
    await assert.rejects(httpRequest(`${base}/ok`, { headers: { [header]: "x" } }), /honest requests only/);
  }
  assert.equal(retryAfterMs("5"), 5000);
  assert.equal(retryAfterMs("999999"), 30_000, "capped");
  assert.equal(retryAfterMs(new Date(Date.now() + 2000).toUTCString(), Date.now()) !== null, true);
  assert.equal(retryAfterMs("soon"), null);
  console.log("http ok");
} finally {
  server.close();
}
