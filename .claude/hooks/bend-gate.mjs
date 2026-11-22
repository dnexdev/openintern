#!/usr/bin/env node
// Claude Code hooks that keep AI edits inside the Bend proofs.
//
//   bend-gate.mjs laws   PreToolUse: ask before anything edits LAWS.bend (the human-owned spec)
//   bend-gate.mjs post   PostToolUse: re-check the proofs after a .bend file changes
//   bend-gate.mjs stop   Stop: when proven code changed, refuse to finish until the full gate passes
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const input = (() => {
  try {
    return JSON.parse(fs.readFileSync(0, "utf8") || "{}");
  } catch {
    return {};
  }
})();
const toolInput = input.tool_input ?? {};

function gate(mode) {
  const r = spawnSync(process.execPath, [path.join(root, "scripts/proofs.mjs"), mode], {
    cwd: root,
    encoding: "utf8",
  });
  return { ok: r.status === 0, out: `${r.stdout ?? ""}${r.stderr ?? ""}`.trim() };
}

function editedPath() {
  return String(toolInput.file_path ?? toolInput.notebook_path ?? "");
}

const mode = process.argv[2];

if (mode === "laws") {
  const command = String(toolInput.command ?? "");
  const touchesLaws =
    path.basename(editedPath()) === "LAWS.bend" ||
    (/LAWS\.bend/.test(command) && /(sed\s+-i|>|\btee\b|\bmv\b|\bcp\b|\brm\b|python|perl|node\s+-e)/.test(command));
  if (touchesLaws) {
    process.stdout.write(
      JSON.stringify({
        hookSpecificOutput: {
          hookEventName: "PreToolUse",
          permissionDecision: "ask",
          permissionDecisionReason:
            "LAWS.bend is the human-owned spec. Approve only if you meant to change a law, not to make a proof pass.",
        },
      }),
    );
  }
  process.exit(0);
}

if (mode === "post") {
  if (!editedPath().endsWith(".bend")) process.exit(0);
  const { ok, out } = gate("prove");
  if (ok) process.exit(0);
  process.stderr.write(`Bend proofs no longer check after this edit:\n${out}\n`);
  process.exit(2);
}

if (mode === "stop") {
  const status = spawnSync(
    "git",
    ["status", "--porcelain", "--", "LAWS.bend", "PROOF.bend", "packages/proven"],
    { cwd: root, encoding: "utf8" },
  );
  if (!status.stdout?.trim()) process.exit(0);
  const { ok, out } = gate("check");
  if (ok) process.exit(0);
  if (input.stop_hook_active) {
    // Already blocked once this turn: tell the user instead of looping.
    process.stdout.write(JSON.stringify({ systemMessage: `Bend proof gate is FAILING:\n${out}` }));
    process.exit(0);
  }
  process.stderr.write(
    `The Bend proof gate fails, so this work is not done. Fix the code or proofs (never LAWS.bend), then run pnpm proofs:build:\n${out}\n`,
  );
  process.exit(2);
}

process.exit(0);
