// Regenerates internal/tools/testdata/golden/*.json from the TypeScript
// scripts. Run from the repository root:
//   node internal/tools/testdata/ts/gen.mjs [case-name...]
// Each case runs in a scratch tree under /tmp/openintern-tools/ (a copy of
// packages/ingest/src, so the scripts' repo-relative data/companies is the
// case's test registry).
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";

const repo = process.cwd();
const td = path.join(repo, "internal/tools/testdata");
const scratch = "/tmp/openintern-tools";
const cases = JSON.parse(fs.readFileSync(path.join(td, "cases.json"), "utf8"));
const only = process.argv.slice(2);
fs.mkdirSync(path.join(td, "golden"), { recursive: true });

const readTree = (dir) => {
  const out = {};
  if (!fs.existsSync(dir)) return out;
  for (const f of fs.readdirSync(dir).sort()) out[f] = fs.readFileSync(path.join(dir, f), "utf8");
  return out;
};

for (const tc of cases) {
  if (only.length && !only.includes(tc.name)) continue;
  const tree = path.join(scratch, "tree-" + tc.name);
  fs.rmSync(tree, { recursive: true, force: true });
  const ingest = path.join(tree, "packages/ingest");
  fs.mkdirSync(ingest, { recursive: true });
  fs.cpSync(path.join(repo, "packages/ingest/src"), path.join(ingest, "src"), { recursive: true });
  fs.copyFileSync(path.join(repo, "packages/ingest/package.json"), path.join(ingest, "package.json"));
  fs.symlinkSync(path.join(repo, "packages/ingest/node_modules"), path.join(ingest, "node_modules"));
  const companies = path.join(tree, "data/companies");
  const reg = path.join(td, tc.registry ?? "registry", "companies");
  if (tc.baseRegistry) {
    fs.cpSync(path.join(td, tc.baseRegistry, "companies"), companies, { recursive: true });
    const git = (...a) => execFileSync("git", a, { cwd: tree, stdio: "pipe" });
    git("init", "-q");
    git("add", "data");
    git("-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "base");
    git("update-ref", "refs/remotes/origin/main", "HEAD");
    fs.rmSync(companies, { recursive: true });
  }
  fs.cpSync(reg, companies, { recursive: true });
  const before = readTree(companies);

  const casePath = path.join(tree, "case.json");
  const resultPath = path.join(tree, "result.json");
  fs.writeFileSync(casePath, JSON.stringify(tc));
  const env = { PATH: process.env.PATH, HOME: process.env.HOME, TZ: "UTC", ...(tc.env ?? {}) };
  try {
    execFileSync(
      path.join(repo, "packages/ingest/node_modules/.bin/tsx"),
      [path.join(td, "ts/run.mts"), path.join(ingest, "src", tc.script + ".ts"), path.join(td, "recordings"), casePath, resultPath],
      { cwd: ingest, env, stdio: "pipe" },
    );
  } catch (e) {
    if (!fs.existsSync(resultPath)) throw e;
  }
  const result = JSON.parse(fs.readFileSync(resultPath, "utf8"));
  const after = readTree(companies);
  const files = {};
  for (const [f, text] of Object.entries(after)) if (before[f] !== text) files[f] = text;
  fs.mkdirSync(path.join(scratch, "requests"), { recursive: true });
  fs.writeFileSync(path.join(scratch, "requests", tc.name + ".txt"), result.requests.join("\n") + "\n");
  const golden = {
    stdout: result.stdout,
    stderr: result.stderr,
    code: result.code,
    requests: [...result.requests].sort(),
    files,
  };
  fs.writeFileSync(path.join(td, "golden", tc.name + ".json"), JSON.stringify(golden, null, 1) + "\n");
  console.log(`${tc.name}: exit ${golden.code}, ${golden.requests.length} requests, ${Object.keys(files).length} files changed`);
}
