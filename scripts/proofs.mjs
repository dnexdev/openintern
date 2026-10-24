#!/usr/bin/env node
// Bend proof gate.
//
//   node scripts/proofs.mjs check   PROOF.bend proves every law in LAWS.bend,
//                                   and generated/proven.mjs is up to date
//   node scripts/proofs.mjs prove   only the proofs (no proven.mjs freshness check)
//   node scripts/proofs.mjs build   check the proofs, then regenerate proven.mjs
//
// Runs `bend` with telemetry and crash reporting off. Uses `--verdict` (the
// Lean-checked kernel) when `lean` is on PATH or BEND_VERDICT=1.
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

function onPath(cmd) {
  return spawnSync("sh", ["-c", `command -v ${cmd}`], { stdio: "ignore" }).status === 0;
}

function bend(args) {
  const r = spawnSync(bendBin(), args, { cwd: root, env, encoding: "utf8" });
  if (r.error) fail(`could not run bend (${r.error.message}); install it or set BEND_BIN`);
  return { status: r.status, out: `${r.stdout ?? ""}${r.stderr ?? ""}` };
}

// Every escape hatch from the proofs is refused outright: @unsafe defs,
// foreign C/JS code, and packages fetched from the hub.
function guard() {
  const files = [...proofFiles, ...fs.readdirSync(bendDir).map((f) => path.join(bendDir, f))];
  const problems = [];
  for (const file of files) {
    if (!file.endsWith(".bend")) continue;
    const rel = path.relative(root, file);
    fs.readFileSync(file, "utf8")
      .split("\n")
      .forEach((line, i) => {
        const at = `${rel}:${i + 1}`;
        if (/@unsafe|^\s*def\s+[\w.]+\?\(/.test(line)) problems.push(`${at} skips the termination check`);
        if (/^\s*import\s+"/.test(line)) problems.push(`${at} imports foreign code`);
        if (/^\s*import\s+(0x|[\w-]+@)/.test(line)) problems.push(`${at} imports a hub package`);
      });
  }
  if (problems.length) fail(`refusing unproven code:\n  ${problems.join("\n  ")}`);
}

function check() {
  guard();
  const verdict = process.env.BEND_VERDICT === "1" || onPath("lean");
  const { out } = bend(verdict ? ["PROOF.bend", "--verdict"] : ["PROOF.bend"]);
  process.stdout.write(out);
  if (!/^ALL PROOFS CHECK/m.test(out) || /SOME PROOFS FAIL/.test(out)) fail("proofs do not check");
  if (!verdict) console.log("proofs: checked by bend2 (install Lean v4.34.0 for --verdict)");
}

function compile(to) {
  const { status, out } = bend(["PROOF.bend", "-o", to]);
  if (status !== 0) fail(`build failed:\n${out}`);
  const js = fs.readFileSync(to, "utf8");
  if (/\bfetch\(|https?:\/\//.test(js)) fail("compiled module contains network code");
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
  fail(`unknown mode ${mode}; use check or build`);
}
