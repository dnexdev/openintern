// Builds the dump-gate fixtures and records what scripts/dump-gate.mjs says
// about each. Usage (repo root): node internal/dump/testdata/gen-gate.mjs
import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";

const root = path.resolve(path.dirname(new URL(import.meta.url).pathname), "gate");
fs.rmSync(root, { recursive: true, force: true });

const job = (i, ats, slug, url) => ({ id: `00000000-0000-4000-8000-${String(i).padStart(12, "0")}`, company: { slug, ats }, apply_url: url });
const ok = (i, ats = "ashby") => job(i, ats, "1password", `https://jobs.ashbyhq.com/1password/${i}`);
const jobsOf = (counts, start = 0) => {
  const out = [];
  let i = start;
  for (const [ats, n] of Object.entries(counts)) for (let k = 0; k < n; k++) out.push(ok(i++, ats));
  return out;
};

function write(dir, jobs, { tamper = false, extra = {} } = {}) {
  fs.mkdirSync(dir, { recursive: true });
  const files = {
    "jobs.json": JSON.stringify({ generatedAt: "2026-11-14T12:00:00.000Z", count: jobs.length, jobs }, null, 2),
    "denylist.json": JSON.stringify({ generatedAt: "x", jobs: [{ id: "cbb76a8d-0726-4a2a-b84d-8f359be23a90", reason: "r" }] }, null, 2),
    ...extra,
  };
  const manifest = {};
  for (const [name, body] of Object.entries(files)) {
    fs.writeFileSync(path.join(dir, name), body);
    manifest[name] = { sha256: createHash("sha256").update(body).digest("hex"), bytes: Buffer.byteLength(body) };
  }
  if (tamper) manifest["jobs.json"].bytes += 1;
  fs.writeFileSync(path.join(dir, "MANIFEST.json"), JSON.stringify({ files: manifest }, null, 2));
}

const cases = [];
const add = (name, newJobs, prevJobs, opts = {}, env = {}) => {
  write(path.join(root, name, "new"), newJobs, opts);
  if (prevJobs) write(path.join(root, name, "prev"), prevJobs);
  cases.push({ name, prev: !!prevJobs, env });
};

add("ok", jobsOf({ ashby: 20, lever: 10 }), jobsOf({ ashby: 20, lever: 10 }));
add("no-prev", jobsOf({ ashby: 3 }), null);
add("tampered", jobsOf({ ashby: 3 }), null, { tamper: true });
add("delisted", [...jobsOf({ ashby: 2 }), { ...ok(9), id: "CBB76A8D-0726-4A2A-B84D-8F359BE23A90" }], null);
const offList = [
  ...jobsOf({ ashby: 2 }),
  job(10, "lever", "1password", "http://jobs.ashbyhq.com/1password/10"),
  job(11, "lever", "no-such-company", "https://jobs.lever.co/x/11"),
  job(12, "lever", "1password", "https://evil.example/1password/12"),
  job(13, "lever", "1password", "https://user@jobs.ashbyhq.com/1password/13"),
  job(14, "lever", "1password", "https://JOBS.ashbyhq.com/1Password/14/"),
  job(15, "lever", "1password", "not a url"),
];
add("off-list-warn", offList, null);
add("off-list-enforce", offList, null, {}, { OPENINTERN_APPLY_HOSTS: "enforce" });
add("breaker", jobsOf({ greenhouse: 25, ashby: 19, workday: 2 }), jobsOf({ greenhouse: 40, lever: 6, ashby: 20, workday: 4 }));
add("breaker-small", jobsOf({ greenhouse: 30, lever: 0 }), jobsOf({ greenhouse: 41, lever: 4 }));
add("growth", jobsOf({ ashby: 30 }), jobsOf({ ashby: 20 }));

const results = [];
for (const c of cases) {
  const args = ["scripts/dump-gate.mjs", path.join(root, c.name, "new")];
  if (c.prev) args.push(path.join(root, c.name, "prev"));
  const r = spawnSync("node", args, { env: { PATH: process.env.PATH, ...c.env }, encoding: "utf8" });
  results.push({ name: c.name, prev: c.prev, env: c.env, code: r.status, stdout: r.stdout, stderr: r.stderr.replaceAll(root, "<root>") });
}
fs.writeFileSync(path.join(root, "results.json"), JSON.stringify({ today: new Date().toISOString().slice(0, 10), results }, null, 2) + "\n");
console.log(results.map((r) => `${r.name}: ${r.code}`).join("\n"));
