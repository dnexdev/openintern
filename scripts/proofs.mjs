#!/usr/bin/env node
// Bend proof gate.
//
//   node scripts/proofs.mjs check   PROOF.bend proves every law in LAWS.bend,
//                                   and generated/proven.mjs is up to date
//   node scripts/proofs.mjs prove   only the proofs (no proven.mjs freshness check)
//   node scripts/proofs.mjs build   check the proofs, then regenerate proven.mjs
//
// Runs `bend` with telemetry and crash reporting off. A pass needs exit status
// 0 and exactly bend2's success output. BEND_VERDICT=1 adds `--verdict`, which
// rechecks non-Base defs with a kernel whose consistency is proven in Lean
// (needs Lean v4.34.0; not run in CI).
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const bendDir = path.join(root, "packages/proven/bend");
const generated = path.join(root, "packages/proven/generated/proven.mjs");
const proofFiles = ["LAWS.bend", "PROOF.bend"].map((f) => path.join(root, f));

const env = {
  ...process.env,
  BEND_NO_TELEMETRY: "1",
  DO_NOT_TRACK: "1",
  BUN_ENABLE_CRASH_REPORTING: "0",
};

function fail(msg) {
  console.error(`proofs: ${msg}`);
  process.exit(1);
}

function bendBin() {
  if (process.env.BEND_BIN) return process.env.BEND_BIN;
  const local = path.join(os.homedir(), ".bend/bin/bend");
  if (fs.existsSync(local)) return local;
  return "bend";
}

function bend(args) {
  const r = spawnSync(bendBin(), args, { cwd: root, env, encoding: "utf8" });
  if (r.error) fail(`could not run bend (${r.error.message}); install it or set BEND_BIN`);
  return { status: r.status, out: `${r.stdout ?? ""}${r.stderr ?? ""}` };
}

// Every escape hatch from the proofs is refused outright: @unsafe defs,
// foreign C/JS code, hub packages, imports from outside the proof tree, and any
// `main` or IO, which could run code that prints a fake pass line.
function guard() {
  const files = [...proofFiles, ...fs.readdirSync(bendDir).map((f) => path.join(bendDir, f))]
    .filter((f) => f.endsWith(".bend"));
  const allowed = new Set(files.map((f) => path.resolve(f)));
  const problems = [];
  for (const file of files) {
    const rel = path.relative(root, file);
    fs.readFileSync(file, "utf8")
      .split("\n")
      .forEach((line, i) => {
        const at = `${rel}:${i + 1}`;
        if (/@unsafe|^\s*def\s+[\w.]+\?\(/.test(line)) problems.push(`${at} skips the termination check`);
        if (/^\s*import\s+"/.test(line)) problems.push(`${at} imports foreign code`);
        if (/^\s*import\s+(0x|[\w-]+@)/.test(line)) problems.push(`${at} imports a hub package`);
        if (/^\s*def\s+main\b/.test(line)) problems.push(`${at} defines main`);
        if (/\bIO\s*[(<.]/.test(line)) problems.push(`${at} uses IO`);
        const local = line.match(/^\s*import\s+(\.\.?\/\S+)/);
        if (local && !allowed.has(path.resolve(path.dirname(file), local[1]))) {
          problems.push(`${at} imports ${local[1]}, outside the proof tree`);
        }
      });
  }
  if (problems.length) fail(`refusing unproven code:\n  ${problems.join("\n  ")}`);
}

const PASS = ["ALL PROOFS CHECK", "Use --verdict for mathematical validity."];

function check() {
  guard();
  const verdict = process.env.BEND_VERDICT === "1";
  const { status, out } = bend(verdict ? ["PROOF.bend", "--verdict"] : ["PROOF.bend"]);
  process.stdout.write(out);
  const lines = out.trim().split("\n").map((l) => l.trim());
  const passed = verdict
    ? lines.length === 1 && lines[0] === PASS[0]
    : lines.length === PASS.length && lines.every((l, i) => l === PASS[i]);
  if (status !== 0 || !passed) fail(`proofs do not check (exit status ${status})`);
}

function compile(to) {
  const { status, out } = bend(["PROOF.bend", "-o", to]);
  if (status !== 0) fail(`build failed:\n${out}`);
}

const mode = process.argv[2] ?? "check";
if (mode === "check") {
  check();
  const tmp = path.join(fs.mkdtempSync(path.join(os.tmpdir(), "proven-")), "proven.mjs");
  compile(tmp);
  const fresh = fs.readFileSync(tmp, "utf8") === fs.readFileSync(generated, "utf8");
  fs.rmSync(path.dirname(tmp), { recursive: true, force: true });
  if (!fresh) fail("packages/proven/generated/proven.mjs is stale; run pnpm proofs:build");
  console.log("proofs: proven.mjs is up to date");
} else if (mode === "prove") {
  check();
} else if (mode === "build") {
  check();
  compile(generated);
  console.log(`proofs: wrote ${path.relative(root, generated)}`);
} else {
  fail(`unknown mode ${mode}; use check, prove or build`);
}
