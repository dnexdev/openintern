import assert from "node:assert/strict";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { writeDumpsFromState } from "./dump-state.js";
import { runIngestFiles } from "./ingest-files.js";
import { loadState, pgTimestamp, validateState, writeState, type State } from "./state.js";

let board: unknown[] = [];
globalThis.fetch = (async (input: string | URL | Request) => {
  if (String(input).includes("boards-api.greenhouse.io/v1/boards/khanacademy")) {
    return new Response(JSON.stringify({ jobs: board }), { status: 200 });
  }
  return new Response("{}", { status: 404 });
}) as typeof fetch;

const intern = { id: 1, title: "Software Engineer Intern (Summer 2027)", absolute_url: "https://job-boards.greenhouse.io/khanacademy/jobs/1", content: "Software engineering internship, React." };
const designer = { id: 2, title: "Senior Product Designer", absolute_url: "https://job-boards.greenhouse.io/khanacademy/jobs/2", content: "Design role." };

// New rows use Postgres's row_to_json timestamp format, so a rollback round trip is byte-identical.
assert.equal(pgTimestamp(new Date("2026-11-21T10:00:00.500Z")), "2026-11-21T10:00:00.5+00:00");
assert.equal(pgTimestamp(new Date("2026-11-21T10:00:00.000Z")), "2026-11-21T10:00:00+00:00");
assert.equal(pgTimestamp(new Date("2026-11-21T10:00:00.010Z")), "2026-11-21T10:00:00.01+00:00");
assert.equal(pgTimestamp(new Date("2026-11-21T10:00:00.123Z")), "2026-11-21T10:00:00.123+00:00");

const dir = await fs.mkdtemp(path.join(os.tmpdir(), "openintern-state-"));
const only = new Set(["khan-academy"]);
const day = 24 * 60 * 60 * 1000;
const t0 = new Date("2026-11-01T12:00:00Z");
let state: State = { companies: new Map(), jobs: new Map(), runs: [] };

board = [intern, designer];
let s = await runIngestFiles(state, { now: t0, only });
assert.equal(s.jobsUpserted, 1, "one internship ingested");
assert.equal(s.lastSeenSweepSkipped, true, "first run ever skips the sweep");
await writeState(dir, state);

state = await loadState(dir);
const job = [...state.jobs.values()][0]!;
assert.equal(job.is_active, true);
const firstSeen = job.first_seen_at;
assert.match(firstSeen, /^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d*[1-9])?\+00:00$/, "Postgres timestamp format");

board = [designer]; // #24: the internship closed
s = await runIngestFiles(state, { now: new Date(t0.getTime() + day), only, syncRegistry: false });
assert.equal(s.jobsDeactivated, 1, "closed posting retired");
await writeState(dir, state);
state = await loadState(dir);
assert.equal(state.jobs.get(job.id)!.is_active, false);
assert.equal(state.jobs.get(job.id)!.first_seen_at, firstSeen, "first_seen_at kept byte for byte");

board = [intern]; // reposted: same id, same first_seen_at
await runIngestFiles(state, { now: new Date(t0.getTime() + 2 * day), only, syncRegistry: false });
assert.equal(state.jobs.size, 1, "same fingerprint, same job");
assert.equal(state.jobs.get(job.id)!.is_active, true);
assert.equal(state.runs.length, 3);
await writeState(dir, state);

const jobsFile = await fs.readFile(path.join(dir, "jobs.jsonl"), "utf8");
assert.equal(jobsFile.includes("last_seen_at"), false, "per-run timestamps stay out of jobs.jsonl");

const out = path.join(dir, "dump");
const r = await writeDumpsFromState(state, out, new Date(t0.getTime() + 2 * day));
assert.equal(r.count, 1);
const manifest = JSON.parse(await fs.readFile(path.join(out, "MANIFEST.json"), "utf8"));
assert.equal(manifest.jobs, 1);
const health = JSON.parse(await fs.readFile(path.join(out, "health.json"), "utf8"));
assert.equal(health.recent_ingest_runs.length, 3);

// Two active jobs with the same URL prefix in one company are refused.
const [a] = [...state.jobs.values()];
state.jobs.set("x", { ...a!, id: `${a!.id.slice(0, 8)}-ffff-4fff-8fff-ffffffffffff`, fingerprint: "other" });
assert.match(validateState(state).join(), /share a URL prefix/);
await assert.rejects(writeState(dir, state), /refusing to write state/);

await fs.rm(dir, { recursive: true });
console.log("state ok");
