/**
 * CI helper: HEAD/GET-check board tokens for YAML files changed in a PR,
 * or all companies when run with --all.
 *
 * Usage:
 *   tsx src/validate-tokens.ts [--all] [file.yaml ...]
 */
import { execFile } from "node:child_process";
import { existsSync } from "node:fs";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";
import YAML from "yaml";
import { companiesFileSchema } from "./schema.js";
import { probeAtsBoard } from "./probe-url.js";
import { defaultCompaniesDir } from "./sync-companies.js";

const repoRoot = fileURLToPath(new URL("../../..", import.meta.url));
const execFileAsync = promisify(execFile);

type CompanyRecord = {
  name: string;
  slug: string;
  ats: string;
  board_token: string;
  active: boolean;
};

/**
 * CI passes repo-root paths (`data/companies/tech-e.yaml`). `pnpm --filter`
 * runs this script with cwd `packages/ingest`, so those paths do not exist
 * relative to cwd. Fall back to the repo root.
 */
function resolveCompanyFile(file: string): string {
  if (path.isAbsolute(file)) return file;
  const fromCwd = path.resolve(file);
  if (existsSync(fromCwd)) return fromCwd;
  return path.resolve(repoRoot, file);
}

/** Proprietary boards that may 403 without dump/browser gates. */
const DUMP_GATED_ATS = new Set([
  "citadel",
  "citadel_securities",
  "tesla",
  "x",
  "avature",
  "icims",
]);

function isDumpGateBlock(ats: string, err: string): boolean {
  return DUMP_GATED_ATS.has(ats) && /blocked|HTTP 403|OPENINTERN_/i.test(err);
}

function parseCompanies(raw: string): CompanyRecord[] {
  return companiesFileSchema.parse(YAML.parse(raw)).companies;
}

async function loadFromFiles(files: string[]): Promise<CompanyRecord[]> {
  const companies: CompanyRecord[] = [];
  for (const file of files) {
    const raw = await fs.readFile(resolveCompanyFile(file), "utf8");
    companies.push(...parseCompanies(raw));
  }
  return companies;
}

/** Identity of the board a company points at. Active defaults to true. */
function boardKey(company: CompanyRecord): string {
  return `${company.ats}\0${company.board_token}\0${company.active !== false}`;
}

/**
 * A PR that touches a company file re-checks every board in it. Tokens that
 * already 404 on the base, and rate limits for boards the PR did not change,
 * are not this PR's failure. Only added or edited active boards are probed.
 */
async function companiesChangedSince(
  files: string[],
  rev: string,
): Promise<CompanyRecord[]> {
  const changed: CompanyRecord[] = [];
  for (const file of files) {
    const abs = resolveCompanyFile(file);
    const rel = path.relative(repoRoot, abs);
    const head = parseCompanies(await fs.readFile(abs, "utf8"));
    let base: CompanyRecord[] = [];
    try {
      const { stdout } = await execFileAsync("git", ["show", `${rev}:${rel}`], {
        cwd: repoRoot,
        maxBuffer: 20 * 1024 * 1024,
      });
      base = parseCompanies(stdout);
    } catch {
      base = [];
    }
    const previous = new Map(base.map((company) => [company.slug, boardKey(company)]));
    for (const company of head) {
      if (company.active === false) continue;
      if (previous.get(company.slug) !== boardKey(company)) changed.push(company);
    }
  }
  return changed;
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

  const baseRef = process.env.GITHUB_BASE_REF;
  const companies = baseRef
    ? await companiesChangedSince(targets, `origin/${baseRef}`)
    : (await loadFromFiles(targets)).filter((c) => c.active);
  const failures: string[] = [];
  const skipped: string[] = [];

  if (companies.length === 0) {
    console.log(
      baseRef
        ? "No added or edited company tokens to validate."
        : "No active companies to validate.",
    );
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
