/**
 * CI helper: HEAD/GET-check board tokens for YAML files changed in a PR,
 * or all companies when run with --all.
 *
 * Usage:
 *   tsx src/validate-tokens.ts [--all] [file.yaml ...]
 */
import fs from "node:fs/promises";
import path from "node:path";
import YAML from "yaml";
import { companiesFileSchema } from "./schema.js";
import { countJobsFromProbeBody, probeUrl } from "./probe-url.js";
import { defaultCompaniesDir } from "./sync-companies.js";

type Probe = { ats: string; token: string; url: string };

async function check(probe: Probe): Promise<string | null> {
  try {
    const res = await fetch(probe.url, {
      method: "GET",
      headers: {
        Accept: "application/json",
        "User-Agent": "OpenIntern/0.1 (+https://github.com/dnexdev/openintern)",
      },
    });
    if (res.status === 404) return `HTTP 404 for ${probe.url}`;
    if (!res.ok && res.status !== 429) return `HTTP ${res.status} for ${probe.url}`;
    return null;
  } catch (err) {
    return err instanceof Error ? err.message : String(err);
  }
}

async function loadFromFiles(files: string[]) {
  const companies: { name: string; slug: string; ats: string; board_token: string }[] = [];
  for (const file of files) {
    const raw = await fs.readFile(file, "utf8");
    const data = companiesFileSchema.parse(YAML.parse(raw));
    companies.push(...data.companies);
  }
  return companies;
}

async function main() {
  const args = process.argv.slice(2);
  const all = args.includes("--all");
  const files = args.filter((a) => !a.startsWith("--"));

  let targets: string[] = files;
  if (all || targets.length === 0) {
    const dir = defaultCompaniesDir();
    const entries = await fs.readdir(dir);
    targets = entries
      .filter((f) => f.endsWith(".yaml") || f.endsWith(".yml"))
      .map((f) => path.join(dir, f));
  }

  if (targets.length === 0) {
    console.log("No company YAML files to validate.");
    return;
  }

  const companies = await loadFromFiles(targets);
  const failures: string[] = [];

  // Cap concurrency
  const CHUNK = 10;
  for (let i = 0; i < companies.length; i += CHUNK) {
    const chunk = companies.slice(i, i + CHUNK);
    const results = await Promise.all(
      chunk.map(async (c) => {
        const err = await check({
          ats: c.ats,
          token: c.board_token,
          url: probeUrl(c.ats, c.board_token),
        });
        return { slug: c.slug, err };
      }),
    );
    for (const r of results) {
      if (r.err) failures.push(`${r.slug}: ${r.err}`);
      else console.log(`ok ${r.slug}`);
    }
  }

  if (failures.length) {
    console.error("\nToken validation failures:");
    for (const f of failures) console.error(`  - ${f}`);
    process.exitCode = 1;
  } else {
    console.log(`\nValidated ${companies.length} companies.`);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
