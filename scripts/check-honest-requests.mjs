#!/usr/bin/env node
// CI gate: ingest makes honest requests only. Fails on a browser user-agent,
// a forged Referer/Origin, cookies, or browser automation anywhere in ingest.
// The single exception is ByteDance's legacy request, which stays only until
// the owner decides on the shadow test (see packages/ingest/src/bytedance.ts).
import fs from "node:fs";
import path from "node:path";

const root = path.resolve(path.dirname(new URL(import.meta.url).pathname), "..");
const dir = path.join(root, "packages/ingest/src");
const files = fs.readdirSync(dir).filter((f) => f.endsWith(".ts") && !f.endsWith(".test.ts"));

const rules = [
  { re: /Mozilla\/5\.0|AppleWebKit|Chrome\/\d/, why: "browser user-agent" },
  { re: /playwright|puppeteer|selenium|chromium/i, why: "browser automation" },
  { re: /["']?\bcookie["']?\s*:/i, why: "cookie header" },
  { re: /["']?\breferer["']?\s*:/i, why: "Referer header", allow: ["bytedance.ts"] },
  { re: /^\s*["']?origin["']?\s*:\s*(portal\.|board\.|`|["'])/im, why: "Origin header", allow: ["bytedance.ts"] },
  // User-Agent may only be one of the OpenIntern constants below.
  { re: /["']User-Agent["']\s*:(?!\s*(?:UA|ua|OPENINTERN_UA)\b)/, why: "custom User-Agent (use http.ts)" },
  { re: /const UA\s*=(?!\s*(?:"OpenIntern\/|OPENINTERN_UA\b))/, why: "UA constant must identify as OpenIntern" },
];

const problems = [];
for (const f of files) {
  const text = fs.readFileSync(path.join(dir, f), "utf8");
  text.split("\n").forEach((line, i) => {
    if (line.trim().startsWith("*") || line.trim().startsWith("//")) return;
    for (const r of rules) {
      if (r.allow?.includes(f)) continue;
      if (r.re.test(line)) problems.push(`${f}:${i + 1}: ${r.why}: ${line.trim()}`);
    }
  });
}
const bytedance = fs.readFileSync(path.join(dir, "bytedance.ts"), "utf8");
if (/referer|origin:/i.test(bytedance) && !bytedance.includes("PENDING OWNER DECISION")) {
  problems.push("bytedance.ts: forged headers without the PENDING OWNER DECISION marker");
}

if (problems.length) {
  console.error(`honest-requests check failed:\n${problems.join("\n")}`);
  process.exit(1);
}
console.log(`honest-requests: ${files.length} ingest files clean`);
