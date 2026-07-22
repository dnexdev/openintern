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
import { probeAtsBoard } from "./probe-url.js";
import { defaultCompaniesDir } from "./sync-companies.js";

/** Proprietary boards that may 403 without dump/browser gates. */
const DUMP_GATED_ATS = new Set(["citadel", "citadel_securities", "tesla"]);

function isDumpGateBlock(ats: string, err: string): boolean {
  return DUMP_GATED_ATS.has(ats) && /blocked|HTTP 403|OPENINTERN_/i.test(err);
}

async function loadFromFiles(files: string[]) {
  const companies: {
    name: string;
    slug: string;
    ats: string;
    board_token: string;
    active: boolean;
  }[] = [];
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

  const companies = (await loadFromFiles(targets)).filter((c) => c.active);
  const failures: string[] = [];
  const skipped: string[] = [];

  if (companies.length === 0) {
    console.log("No active companies to validate.");
    return;
  }

  const inCi = process.env.CI === "true" || process.env.CI === "1";

  const CHUNK = 10;
  for (let i = 0; i < companies.length; i += CHUNK) {
    const chunk = companies.slice(i, i + CHUNK);
    const results = await Promise.all(
      chunk.map(async (c) => {
        const result = await probeAtsBoard(c.ats, c.board_token);
        return {
          slug: c.slug,
          ats: c.ats,
          err: result.ok ? null : result.error ?? `HTTP ${result.status}`,
        };
      }),
    );
    for (const r of results) {
      if (!r.err) {
        console.log(`ok ${r.slug}`);
        continue;
      }
      // CI runners often cannot reach Akamai-gated careers sites without dumps/browser.
      if (inCi && isDumpGateBlock(r.ats, r.err)) {
        skipped.push(`${r.slug}: ${r.err}`);
        console.warn(`skip ${r.slug} (dump-gated proprietary in CI)`);
        continue;
      }
      failures.push(`${r.slug}: ${r.err}`);
    }
  }

  if (failures.length) {
    console.error("\nToken validation failures:");
    for (const f of failures) console.error(`  - ${f}`);
    process.exitCode = 1;
  } else {
    console.log(
      `\nValidated ${companies.length - skipped.length} companies` +
        (skipped.length ? ` (${skipped.length} dump-gated skipped in CI)` : "") +
        ".",
    );
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
