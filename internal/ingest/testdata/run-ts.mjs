// Runs every day of both scenarios through the TypeScript and records the
// sha256 of each state file and output after each day (expected.json).
// Usage (repo root): node internal/ingest/testdata/run-ts.mjs <work-dir>
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { gunzipSync } from "node:zlib";

const here = path.dirname(new URL(import.meta.url).pathname);
const work = path.resolve(process.argv[2]);
const scenarios = JSON.parse(fs.readFileSync(path.join(here, "scenarios.json"), "utf8"));
const files = ["companies.jsonl", "jobs.jsonl", "last_seen.tsv", "runs.jsonl"];
const outs = ["summary.json", "digest.md", "step.md"];
const sha = (f) => createHash("sha256").update(fs.readFileSync(f)).digest("hex");
const expected = {};
for (const [name, days] of Object.entries(scenarios)) {
  const dir = path.join(work, name, "state");
  fs.rmSync(path.join(work, name), { recursive: true, force: true });
  fs.mkdirSync(dir, { recursive: true });
  if (name === "state-real") {
    for (const f of files) fs.writeFileSync(path.join(dir, f), gunzipSync(fs.readFileSync(path.join(here, "../../dump/testdata/state-real", f + ".gz"))));
  }
  expected[name] = [];
  days.forEach((_, d) => {
    const out = path.join(work, name, `out${d}`);
    execFileSync("packages/ingest/node_modules/.bin/tsx", ["--no-warnings", "--experimental-test-module-mocks", path.join(here, "ingest-ts.mts"), dir, name, String(d), out], { stdio: ["ignore", "ignore", "inherit"] });
    fs.cpSync(dir, path.join(work, name, `state${d}`), { recursive: true });
    const rec = {};
    for (const f of files) rec[f] = sha(path.join(dir, f));
    for (const f of outs) rec[f] = sha(path.join(out, f));
    expected[name].push(rec);
  });
}
fs.writeFileSync(path.join(here, "expected.json"), JSON.stringify(expected, null, 1) + "\n");
console.log("ok");
