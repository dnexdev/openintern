/**
 * Discovery-only gap report: parse public SpeedyApply / SimplifyJobs README
 * tables and diff company names against data/companies/*.yaml.
 *
 * Does NOT ingest their job rows — only surfaces missing registry companies.
 *
 * Usage:
 *   tsx src/gap-report.ts
 *   tsx src/gap-report.ts --json
 *   tsx src/gap-report.ts --top=30
 */
import fs from "node:fs/promises";
import path from "node:path";
import YAML from "yaml";
import { companiesFileSchema } from "./schema.js";
import { defaultCompaniesDir } from "./sync-companies.js";

const SOURCES = [
  {
    id: "speedyapply-usa",
    url: "https://raw.githubusercontent.com/speedyapply/2027-SWE-College-Jobs/main/README.md",
  },
  {
    id: "speedyapply-intl",
    url: "https://raw.githubusercontent.com/speedyapply/2027-SWE-College-Jobs/main/INTERN_INTL.md",
  },
  {
    id: "simplify-summer",
    url: "https://raw.githubusercontent.com/SimplifyJobs/Summer2026-Internships/dev/README.md",
  },
  {
    id: "simplify-offseason",
    url: "https://raw.githubusercontent.com/SimplifyJobs/Summer2026-Internships/dev/README-Off-Season.md",
  },
] as const;

const UA = "OpenIntern/0.1 (+https://github.com/dnexdev/openintern; gap-report)";

function slugify(name: string): string {
  return name
    .toLowerCase()
    .normalize("NFKD")
    .replace(/[^\w\s-]/g, "")
    .trim()
    .replace(/[\s_]+/g, "-")
    .replace(/-+/g, "-");
}

function normalizeName(name: string): string {
  return name
    .toLowerCase()
    .normalize("NFKD")
    .replace(/&/g, " and ")
    .replace(/[^a-z0-9]+/g, "")
    .trim();
}

function stripHtml(html: string): string {
  return html
    .replace(/<script[\s\S]*?<\/script>/gi, " ")
    .replace(/<style[\s\S]*?<\/style>/gi, " ")
    .replace(/<br\s*\/?>/gi, " ")
    .replace(/<\/?[^>]+>/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/&nbsp;/g, " ")
    .replace(/&#0?39;/g, "'")
    .replace(/&quot;/g, '"')
    // Strip leading emoji / symbols (Simplify FAANG markers)
    .replace(/^[\s\p{Extended_Pictographic}\uFE0F\u200D🔥🔒🛂🇺🇸🎓]+/gu, "")
    .replace(/\s+/g, " ")
    .trim();
}

/** Extract company names from markdown / HTML table rows. */
export function extractCompaniesFromMarkdown(md: string): string[] {
  const found = new Set<string>();

  // HTML tables: first <td> in each data row
  for (const match of md.matchAll(/<tr[^>]*>\s*<td[^>]*>([\s\S]*?)<\/td>/gi)) {
    const cell = stripHtml(match[1]);
    if (isPlausibleCompany(cell)) found.add(cell);
  }

  // SpeedyApply-style: <a href="..."><strong>Company</strong></a>
  for (const match of md.matchAll(
    /<a\s+href=["'][^"']+["'][^>]*>\s*(?:<strong>)?([^<]+?)(?:<\/strong>)?\s*<\/a>/gi,
  )) {
    const cell = stripHtml(match[1]);
    if (isPlausibleCompany(cell)) found.add(cell);
  }

  // Pipe markdown tables: | Company | Position | ...
  for (const line of md.split("\n")) {
    if (!line.includes("|")) continue;
    if (/^\s*\|?\s*:?-{3,}/.test(line)) continue;
    const cells = line
      .split("|")
      .map((c) =>
        stripHtml(c.replace(/\*\*/g, "").replace(/\[([^\]]+)\]\([^)]+\)/g, "$1")),
      )
      .filter(Boolean);
    if (cells.length < 2) continue;
    const first = cells[0]!;
    if (/^company$/i.test(first)) continue;
    if (isPlausibleCompany(first)) found.add(first);
  }

  return [...found];
}

function isPlausibleCompany(name: string): boolean {
  if (name.length < 2 || name.length > 80) return false;
  if (/^(company|position|role|location|salary|posting|age|application|faang)/i.test(name)) {
    return false;
  }
  if (/^https?:/i.test(name)) return false;
  if (/^\d+[dhms]?$/i.test(name)) return false;
  // Skip emoji-only / legend rows
  if (!/[a-zA-Z]/.test(name)) return false;
  return true;
}

async function loadRegistry(dir: string) {
  const entries = await fs.readdir(dir);
  const byNorm = new Map<string, { name: string; slug: string; active: boolean }>();
  const bySlug = new Map<string, { name: string; slug: string; active: boolean }>();

  for (const file of entries.filter((f) => f.endsWith(".yaml") || f.endsWith(".yml"))) {
    const raw = await fs.readFile(path.join(dir, file), "utf8");
    const data = companiesFileSchema.parse(YAML.parse(raw));
    for (const c of data.companies) {
      const row = { name: c.name, slug: c.slug, active: c.active ?? true };
      bySlug.set(c.slug, row);
      byNorm.set(normalizeName(c.name), row);
      byNorm.set(normalizeName(c.slug.replace(/-/g, " ")), row);
    }
  }
  return { byNorm, bySlug };
}

async function fetchText(url: string): Promise<string> {
  const res = await fetch(url, {
    headers: { "User-Agent": UA, Accept: "text/plain,text/markdown,*/*" },
  });
  if (!res.ok) throw new Error(`HTTP ${res.status} for ${url}`);
  return res.text();
}

async function main() {
  const asJson = process.argv.includes("--json");
  const topArg = process.argv.find((a) => a.startsWith("--top="));
  const topN = topArg ? Math.max(1, Number(topArg.split("=")[1]) || 60) : 60;
  const dir = defaultCompaniesDir();
  const registry = await loadRegistry(dir);

  const missing: {
    name: string;
    slug_guess: string;
    sources: string[];
  }[] = [];
  const inactiveHits: {
    name: string;
    slug: string;
    sources: string[];
  }[] = [];
  const seenMissing = new Map<string, { name: string; sources: Set<string> }>();
  const seenInactive = new Map<string, { name: string; slug: string; sources: Set<string> }>();

  let totalExternal = 0;

  for (const src of SOURCES) {
    let md: string;
    try {
      md = await fetchText(src.url);
    } catch (err) {
      console.error(`warn: failed ${src.id}:`, err instanceof Error ? err.message : err);
      continue;
    }
    const names = extractCompaniesFromMarkdown(md);
    totalExternal += names.length;

    for (const name of names) {
      const norm = normalizeName(name);
      const hit = registry.byNorm.get(norm);
      if (hit) {
        if (!hit.active) {
          const prev = seenInactive.get(hit.slug);
          if (prev) prev.sources.add(src.id);
          else
            seenInactive.set(hit.slug, {
              name: hit.name,
              slug: hit.slug,
              sources: new Set([src.id]),
            });
        }
        continue;
      }
      const prev = seenMissing.get(norm);
      if (prev) prev.sources.add(src.id);
      else seenMissing.set(norm, { name, sources: new Set([src.id]) });
    }
  }

  for (const [, v] of [...seenMissing.entries()].sort((a, b) =>
    a[1].name.localeCompare(b[1].name),
  )) {
    missing.push({
      name: v.name,
      slug_guess: slugify(v.name),
      sources: [...v.sources].sort(),
    });
  }
  for (const [, v] of [...seenInactive.entries()].sort((a, b) =>
    a[1].name.localeCompare(b[1].name),
  )) {
    inactiveHits.push({
      name: v.name,
      slug: v.slug,
      sources: [...v.sources].sort(),
    });
  }

  const report = {
    generatedAt: new Date().toISOString(),
    registrySize: registry.bySlug.size,
    externalCompanyMentions: totalExternal,
    missingCount: missing.length,
    inactiveButListedCount: inactiveHits.length,
    top: topN,
    missing: missing.slice(0, topN),
    inactiveButListed: inactiveHits.slice(0, Math.min(topN, 100)),
  };

  if (asJson) {
    console.log(JSON.stringify(report, null, 2));
    return;
  }

  console.log(`Registry: ${report.registrySize} companies`);
  console.log(`External mentions parsed: ${report.externalCompanyMentions}`);
  console.log(`Missing from registry: ${report.missingCount}`);
  console.log(`In registry but inactive (listed externally): ${report.inactiveButListedCount}`);
  console.log("\n## Top missing (add YAML + board token)\n");
  for (const m of report.missing) {
    console.log(`- ${m.name}  (slug: ${m.slug_guess})  [${m.sources.join(", ")}]`);
    console.log(`    # suggested starter:`);
    console.log(`    # - name: ${m.name}`);
    console.log(`    #   slug: ${m.slug_guess}`);
    console.log(`    #   ats: greenhouse  # verify with pnpm validate-tokens`);
    console.log(`    #   board_token: CHANGE_ME`);
    console.log(`    #   website_url: https://www.example.com`);
  }
  console.log("\n## Inactive but still listed externally (recover token?)\n");
  for (const m of report.inactiveButListed.slice(0, 40)) {
    console.log(`- ${m.name}  (${m.slug})  [${m.sources.join(", ")}]`);
  }
}

const isMain =
  process.argv[1] &&
  (process.argv[1].endsWith("gap-report.ts") ||
    process.argv[1].endsWith("gap-report.js"));

if (isMain) {
  main().catch((err) => {
    console.error(err);
    process.exit(1);
  });
}
