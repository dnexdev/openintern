#!/usr/bin/env node
// Write gate for flat-file state. Plain Node, no packages: it runs in the job
// that holds the state repo's deploy key, which installs nothing from npm.
//
//   node scripts/state-gate.mjs <current-state-dir> <candidate-state-dir>
//
// Refuses (exit 1) when the candidate:
//   - is malformed (bad JSON, duplicate ids, slugs or fingerprints, jobs of
//     unknown companies, two active jobs sharing a URL prefix in a company);
//   - changes an existing job's id, company or first_seen_at;
//   - adds more than MAX_NEW_JOBS_PER_COMPANY jobs to one company in one run;
//   - drops a company with at least 5 active jobs to zero, unless
//     data/expected-delta.json approves "company:<slug>".
// It prints apply hosts not seen in the current state for review.
import fs from "node:fs";
import path from "node:path";

const MAX_NEW_JOBS_PER_COMPANY = 200;
const root = path.resolve(path.dirname(new URL(import.meta.url).pathname), "..");
const [curDir, newDir] = process.argv.slice(2);
if (!curDir || !newDir) {
  console.error("usage: state-gate.mjs <current-state-dir> <candidate-state-dir>");
  process.exit(2);
}

const problems = [];
function jsonl(dir, name) {
  const file = path.join(dir, name);
  if (!fs.existsSync(file)) return [];
  return fs
    .readFileSync(file, "utf8")
    .split("\n")
    .filter((l) => l.trim())
    .map((l, i) => {
      try {
        return JSON.parse(l);
      } catch {
        problems.push(`${name}:${i + 1} in ${dir}: invalid JSON`);
        return null;
      }
    })
    .filter(Boolean);
}

const cur = { companies: jsonl(curDir, "companies.jsonl"), jobs: jsonl(curDir, "jobs.jsonl") };
const next = { companies: jsonl(newDir, "companies.jsonl"), jobs: jsonl(newDir, "jobs.jsonl") };

// Structure.
const companyIds = new Map();
const slugs = new Set();
for (const c of next.companies) {
  if (companyIds.has(c.id)) problems.push(`duplicate company id ${c.id}`);
  if (slugs.has(c.slug)) problems.push(`duplicate company slug ${c.slug}`);
  companyIds.set(c.id, c);
  slugs.add(c.slug);
}
const jobIds = new Set();
const fingerprints = new Set();
const prefixes = new Map();
for (const j of next.jobs) {
  if (jobIds.has(j.id)) problems.push(`duplicate job id ${j.id}`);
  if (fingerprints.has(j.fingerprint)) problems.push(`duplicate fingerprint ${j.fingerprint}`);
  jobIds.add(j.id);
  fingerprints.add(j.fingerprint);
  if (!companyIds.has(j.company_id)) problems.push(`job ${j.id} has unknown company ${j.company_id}`);
  if (j.is_active) {
    const key = `${j.company_id} ${j.id.replace(/-/g, "").slice(0, 8)}`;
    if (prefixes.has(key)) problems.push(`active jobs ${prefixes.get(key)} and ${j.id} share a URL prefix`);
    prefixes.set(key, j.id);
  }
}

// Identity never changes.
const before = new Map(cur.jobs.map((j) => [j.id, j]));
for (const j of next.jobs) {
  const b = before.get(j.id);
  if (!b) continue;
  if (b.company_id !== j.company_id) problems.push(`job ${j.id} moved company`);
  if (b.first_seen_at !== j.first_seen_at) problems.push(`job ${j.id} first_seen_at changed`);
}
for (const id of before.keys()) if (!jobIds.has(id)) problems.push(`job ${id} disappeared (jobs are never deleted)`);

// Growth and collapse per company.
const expectedPath = path.join(root, "data/expected-delta.json");
const expected = fs.existsSync(expectedPath) ? JSON.parse(fs.readFileSync(expectedPath, "utf8")) : { approvals: [] };
const today = new Date().toISOString().slice(0, 10);
const approved = (scope) => expected.approvals.some((a) => a.scope === scope && a.until >= today && a.reason?.trim());
const count = (jobs, pred) => {
  const m = new Map();
  for (const j of jobs) if (pred(j)) m.set(j.company_id, (m.get(j.company_id) ?? 0) + 1);
  return m;
};
const added = count(next.jobs, (j) => !before.has(j.id));
for (const [cid, n] of added) {
  if (n > MAX_NEW_JOBS_PER_COMPANY && !approved(`company:${companyIds.get(cid)?.slug}`)) {
    problems.push(`${companyIds.get(cid)?.slug} added ${n} jobs in one run (limit ${MAX_NEW_JOBS_PER_COMPANY})`);
  }
}
const activeBefore = count(cur.jobs, (j) => j.is_active);
const activeAfter = count(next.jobs, (j) => j.is_active);
for (const [cid, n] of activeBefore) {
  const slug = companyIds.get(cid)?.slug ?? cid;
  if (n >= 5 && !activeAfter.get(cid) && companyIds.get(cid)?.active !== false && !approved(`company:${slug}`)) {
    problems.push(`${slug} dropped from ${n} active jobs to 0`);
  }
}

// New apply hosts, for the reviewer.
const hostOf = (u) => {
  try {
    return new URL(u).hostname;
  } catch {
    return "(malformed)";
  }
};
const knownHosts = new Set(cur.jobs.map((j) => hostOf(j.apply_url)));
const newHosts = [...new Set(next.jobs.filter((j) => j.is_active).map((j) => hostOf(j.apply_url)))].filter((h) => !knownHosts.has(h));
if (newHosts.length) console.log(`state-gate: new apply hosts: ${newHosts.sort().join(", ")}`);

if (problems.length) {
  for (const p of problems.slice(0, 50)) console.error(`state-gate: ${p}`);
  console.error(`state-gate: refusing to publish (${problems.length} problem(s))`);
  process.exit(1);
}
console.log(`state-gate: ok (${next.jobs.length} jobs, ${next.companies.length} companies)`);
