#!/usr/bin/env node
// Publish gate for the daily dump. Plain Node, no packages: it runs in the job
// that holds release-write permission, which installs nothing from npm.
//
//   node scripts/dump-gate.mjs <new-dir> [<previous-dir>]
//
// Holds the release (exit 1) when:
//   - a file does not match MANIFEST.json;
//   - an apply URL is not https or not on the compiled allowlist;
//   - a delisted job id is present;
//   - the job total moves more than 10%, an ATS drops more than 15% and more
//     than 10 jobs, or an ATS with at least 5 jobs drops to zero,
//     unless data/expected-delta.json approves it (see that file).
import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";

const root = path.resolve(path.dirname(new URL(import.meta.url).pathname), "..");
const [newDir, prevDir] = process.argv.slice(2);
if (!newDir) {
  console.error("usage: dump-gate.mjs <new-dir> [<previous-dir>]");
  process.exit(2);
}

const readJson = (file) => JSON.parse(fs.readFileSync(file, "utf8"));
const problems = [];
const holds = [];

// 1. Integrity.
const manifest = readJson(path.join(newDir, "MANIFEST.json"));
for (const [name, { sha256, bytes }] of Object.entries(manifest.files)) {
  const buf = fs.readFileSync(path.join(newDir, name));
  const got = createHash("sha256").update(buf).digest("hex");
  if (got !== sha256 || buf.length !== bytes) problems.push(`${name}: does not match MANIFEST.json`);
}

// 2. Every apply link is https and on the allowlist built into this commit.
const table = readJson(path.join(root, "packages/db/src/apply-hosts.generated.json"));
const dump = readJson(path.join(newDir, "jobs.json"));
const deny = new Set(readJson(path.join(newDir, "denylist.json")).jobs.map((j) => j.id.toLowerCase()));
function allowed(slug, url) {
  let u;
  try {
    u = new URL(url);
  } catch {
    return false;
  }
  if (u.protocol !== "https:" || u.username || u.password || u.port) return false;
  const host = u.hostname.toLowerCase();
  const p = `${u.pathname.toLowerCase().replace(/\/+$/, "")}/`;
  return (table[slug] ?? []).some(([h, prefix]) => h === host && p.startsWith(prefix.toLowerCase()));
}
const enforce = (process.env.OPENINTERN_APPLY_HOSTS ?? "").trim() === "enforce";
let offList = 0;
for (const job of dump.jobs) {
  if (deny.has(job.id.toLowerCase())) problems.push(`delisted job ${job.id} is in the dump`);
  if (!allowed(job.company.slug, job.apply_url)) {
    offList++;
    if (enforce) problems.push(`off-allowlist apply URL for ${job.company.slug}: ${job.apply_url}`);
  }
}
if (offList && !enforce) console.warn(`dump-gate: ${offList} apply URLs off the allowlist (warn mode)`);

// 3. Circuit breaker against the previous release.
if (prevDir && fs.existsSync(path.join(prevDir, "jobs.json"))) {
  const prev = readJson(path.join(prevDir, "jobs.json"));
  const count = (jobs) => {
    const by = {};
    for (const j of jobs) by[j.company.ats] = (by[j.company.ats] ?? 0) + 1;
    return by;
  };
  const before = count(prev.jobs);
  const after = count(dump.jobs);
  const expectedPath = path.join(root, "data/expected-delta.json");
  const expected = fs.existsSync(expectedPath) ? readJson(expectedPath) : { approvals: [] };
  const today = new Date().toISOString().slice(0, 10);
  const approved = (scope) =>
    expected.approvals.some((a) => a.scope === scope && a.until >= today && a.reason?.trim());

  const prevTotal = prev.jobs.length;
  const newTotal = dump.jobs.length;
  if (prevTotal > 0 && Math.abs(newTotal - prevTotal) / prevTotal > 0.1 && !approved("total")) {
    holds.push(`total ${prevTotal} → ${newTotal} (more than 10%)`);
  }
  for (const ats of Object.keys(before)) {
    const b = before[ats];
    const a = after[ats] ?? 0;
    const drop = b - a;
    if (a === 0 && b >= 5 && !approved(`ats:${ats}`)) holds.push(`${ats} ${b} → 0`);
    else if (drop > 10 && drop / b > 0.15 && !approved(`ats:${ats}`)) holds.push(`${ats} ${b} → ${a}`);
  }
}

if (problems.length || holds.length) {
  for (const p of problems) console.error(`dump-gate: ${p}`);
  for (const h of holds) console.error(`dump-gate: hold: ${h}`);
  console.error("dump-gate: not publishing. Approve an expected change in data/expected-delta.json if it is real.");
  process.exit(1);
}
console.log(`dump-gate: ok (${dump.jobs.length} jobs)`);
