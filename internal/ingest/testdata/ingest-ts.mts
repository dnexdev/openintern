// One day of the ingest parity scenario through the TypeScript
// runIngestFiles, with a fake fetcher and fake ids.
// Usage (repo root):
//   packages/ingest/node_modules/.bin/tsx --experimental-test-module-mocks \
//     internal/ingest/testdata/ingest-ts.mts <state-dir> <scenario> <day> <out-dir>
import { mock } from "node:test";
import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { syncBuiltinESMExports } from "node:module";
import { gunzipSync } from "node:zlib";

const [stateDir, scenario, dayArg, outDir] = process.argv.slice(2);
const dayIndex = Number(dayArg);
const here = path.dirname(new URL(import.meta.url).pathname);
const src = path.resolve(here, "../../../packages/ingest/src");
const boards = JSON.parse(gunzipSync(fs.readFileSync(path.join(here, "boards.json.gz"))).toString("utf8"));
const day = JSON.parse(fs.readFileSync(path.join(here, "scenarios.json"), "utf8"))[scenario][dayIndex];

// ids: 8 hex digits counting up from a per-day base, so URL prefixes differ.
let n = 0x10000000 + dayIndex * 0x00100000 + (scenario === "empty" ? 0 : 0x40000000);
crypto.randomUUID = (() => `${(n++).toString(16)}-0000-4000-8000-000000000000`) as typeof crypto.randomUUID;
syncBuiltinESMExports();

const { FetchFailure } = await import(path.join(src, "http.ts"));
const drop = new Set<string>(day.drop ?? []);
const empty = new Set<string>(day.empty ?? []);
mock.module(new URL("file://" + path.join(src, "ats.ts")).href, {
  namedExports: {
    PROPRIETARY_ATS: [],
    fetchJobsForAts: async (ats: string, token: string) => {
      const key = `${ats}\t${token}`;
      const err = day.errors?.[key];
      if (err) throw err.kind ? new FetchFailure(err.kind, err.message, err.status) : new Error(err.message);
      if (empty.has(key)) return [];
      return (boards[key] ?? [])
        .filter((j: any) => !drop.has(j.externalId))
        .map((j: any) => ({ ...j, postedAt: j.postedAt == null ? null : new Date(j.postedAt) }));
    },
  },
});
for (const [k, v] of Object.entries(day.env ?? {})) process.env[k] = v as string;

const { loadState, writeState } = await import(path.join(src, "state.ts"));
const { runIngestFiles } = await import(path.join(src, "ingest-files.ts"));
const { ingestDigest, ingestStepSummary } = await import(path.join(src, "ingest.ts"));
const state = await loadState(stateDir);
const only = process.env.OPENINTERN_ONLY_SLUGS?.trim();
const summary = await runIngestFiles(state, {
  now: new Date(day.now),
  syncRegistry: !day.skipSync,
  only: only ? new Set(only.split(",").map((x: string) => x.trim()).filter(Boolean)) : undefined,
});
await writeState(stateDir, state);
fs.mkdirSync(outDir, { recursive: true });
fs.writeFileSync(path.join(outDir, "summary.json"), JSON.stringify(summary, null, 2));
fs.writeFileSync(path.join(outDir, "digest.md"), ingestDigest(summary) ?? "");
fs.writeFileSync(path.join(outDir, "step.md"), ingestStepSummary(summary));
