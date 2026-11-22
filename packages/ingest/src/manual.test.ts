import assert from "node:assert/strict";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { FetchFailure } from "./http.js";
import { MANUAL_MAX_AGE_DAYS, loadManualJobs, toManualCapture, validateManualCapture } from "./manual.js";

const dir = await fs.mkdtemp(path.join(os.tmpdir(), "openintern-manual-"));
const now = new Date("2026-11-01T12:00:00Z");
const day = 24 * 60 * 60 * 1000;

const capture = toManualCapture(
  [
    {
      externalId: "2",
      title: "Software Engineering Intern",
      locations: ["Palo Alto, CA"],
      applyUrl: "https://www.tesla.com/careers/search/job/2",
      excerpt: null,
      postedAt: null,
      description: "x ".repeat(600),
    },
    {
      externalId: "1",
      title: "Firmware Intern",
      locations: [],
      applyUrl: "https://www.tesla.com/careers/search/job/1",
      excerpt: "Embedded C",
      postedAt: new Date("2026-10-30T00:00:00Z"),
      description: "",
    },
  ],
  { source: "https://www.tesla.com/cua-api/apps/careers/state", now },
);
assert.equal(capture.captured_by, "maintainer");
assert.deepEqual(capture.jobs.map((j) => j.external_id), ["1", "2"], "sorted for stable diffs");
assert.equal(capture.jobs[1]!.excerpt!.length <= 400, true, "excerpt capped");

await fs.writeFile(path.join(dir, "tesla.json"), JSON.stringify(capture));
const jobs = await loadManualJobs("tesla", { dir, now: new Date(now.getTime() + 3 * day) });
assert.equal(jobs.length, 2);
assert.equal(jobs[0]!.postedAt?.toISOString(), "2026-10-30T00:00:00.000Z");

await assert.rejects(
  loadManualJobs("tesla", { dir, now: new Date(now.getTime() + (MANUAL_MAX_AGE_DAYS + 1) * day) }),
  (err) => err instanceof FetchFailure && err.kind === "manual-expired",
);
await assert.rejects(
  loadManualJobs("citadel", { dir, now }),
  (err) => err instanceof FetchFailure && err.kind === "not-found",
);
await assert.rejects(loadManualJobs("../etc", { dir, now }), /invalid manual source/);

const bad = { ...capture, jobs: [{ ...capture.jobs[0]!, apply_url: "javascript:alert(1)", excerpt: "y".repeat(401) }] };
assert.throws(() => validateManualCapture(bad, now), /apply_url must be an https URL.*excerpt/);
assert.throws(() => validateManualCapture({ ...capture, captured_at: "2027-01-01T00:00:00Z" }, now), /future/);
assert.throws(
  () => validateManualCapture({ ...capture, jobs: [capture.jobs[0], capture.jobs[0]] }, now),
  /duplicated/,
);

await fs.rm(dir, { recursive: true });
console.log("manual ok");
