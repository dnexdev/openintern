/**
 * Regression for issue #59: dump-first pages must treat age ≤ 30h as fresh
 * (same rule as Go board.MaxDumpAge and /api/v1/status).
 *
 *   packages/ingest/node_modules/.bin/tsx --test apps/web/src/lib/dump-age.test.ts
 */
import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { isDumpStale, MAX_DUMP_AGE_MS } from "./dump-age.ts";

describe("isDumpStale", () => {
  const now = Date.parse("2026-11-27T12:00:00.000Z");

  it("treats a dump at the 30h boundary as fresh", () => {
    const generatedAt = new Date(now - MAX_DUMP_AGE_MS).toISOString();
    assert.equal(isDumpStale(generatedAt, now), false);
  });

  it("treats a dump one ms past 30h as stale", () => {
    const generatedAt = new Date(now - MAX_DUMP_AGE_MS - 1).toISOString();
    assert.equal(isDumpStale(generatedAt, now), true);
  });

  it("treats an unparseable timestamp as stale", () => {
    assert.equal(isDumpStale("not-a-date", now), true);
  });

  it("does not flag a just-published dump", () => {
    assert.equal(isDumpStale(new Date(now).toISOString(), now), false);
  });
});
