/**
 * Discovery gap report: parse SpeedyApply / SimplifyJobs README tables and
 * diff against data/companies/*.yaml.
 *
 * Usage:
 *   tsx src/gap-report.ts
 *   tsx src/gap-report.ts --json
 *   tsx src/gap-report.ts --top=30
 *   tsx src/gap-report.ts --ats              # Apply-URL → ATS token extraction
 *   tsx src/gap-report.ts --ats --write      # also append probe-OK missing to tech-c.yaml
 *   tsx src/gap-report.ts --ats --json
 */
import fs from "node:fs/promises";
import path from "node:path";
import YAML from "yaml";
import { companiesFileSchema, type CompanyYaml } from "./schema.js";
import { defaultCompaniesDir } from "./sync-companies.js";
import {
  extractAtsBoardsFromMarkdown,
  tokenKey,
  type ExtractedAtsHit,
  type SupportedAts,
} from "./parse-ats-url.js";
import { probeAtsBoard } from "./probe-url.js";
import { stripHtml } from "./strip-html.js";

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

/** Sources with markdown Apply links we can parse for ATS tokens. */
const ATS_SOURCES = SOURCES.filter(
  (s) => s.id === "simplify-summer" || s.id === "simplify-offseason",
);

const UA = "OpenIntern/0.1 (+https://github.com/dnexdev/openintern; gap-report)";

export function slugify(name: string): string {
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

/** Extract company names from markdown / HTML table rows. */
export function extractCompaniesFromMarkdown(md: string): string[] {
  const found = new Set<string>();

  for (const match of md.matchAll(/<tr[^>]*>\s*<td[^>]*>([\s\S]*?)<\/td>/gi)) {
    const cell = stripHtml(match[1]);
    if (isPlausibleCompany(cell)) found.add(cell);
  }

  for (const match of md.matchAll(
    /<a\s+href=["'][^"']+["'][^>]*>\s*(?:<strong>)?([^<]+?)(?:<\/strong>)?\s*<\/a>/gi,
  )) {
    const cell = stripHtml(match[1]);
    if (isPlausibleCompany(cell)) found.add(cell);
  }

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
  if (!/[a-zA-Z]/.test(name)) return false;
  return true;
}

type RegistryRow = {
  name: string;
  slug: string;
  active: boolean;
  ats: string;
  boardToken: string;
  file: string;
};

async function loadRegistry(dir: string) {
  const entries = await fs.readdir(dir);
  const byNorm = new Map<string, RegistryRow>();
  const bySlug = new Map<string, RegistryRow>();
  const byToken = new Map<string, RegistryRow>();

  for (const file of entries.filter((f) => f.endsWith(".yaml") || f.endsWith(".yml"))) {
    const raw = await fs.readFile(path.join(dir, file), "utf8");
    const data = companiesFileSchema.parse(YAML.parse(raw));
    for (const c of data.companies) {
      const row: RegistryRow = {
        name: c.name,
        slug: c.slug,
        active: c.active ?? true,
        ats: c.ats,
        boardToken: c.board_token,
        file,
      };
      bySlug.set(c.slug, row);
      byNorm.set(normalizeName(c.name), row);
      byNorm.set(normalizeName(c.slug.replace(/-/g, " ")), row);
      byToken.set(tokenKey(c.ats, c.board_token), row);
    }
  }
  return { byNorm, bySlug, byToken };
}

async function fetchText(url: string): Promise<string> {
  const res = await fetch(url, {
    headers: { "User-Agent": UA, Accept: "text/plain,text/markdown,*/*" },
  });
  if (!res.ok) throw new Error(`HTTP ${res.status} for ${url}`);
  return res.text();
}

async function probeOk(ats: string, token: string): Promise<boolean> {
  const result = await probeAtsBoard(ats, token, UA);
  return result.ok;
}

type AtsCandidate = {
  name: string;
  slug: string;
  ats: SupportedAts;
  boardToken: string;
  careersUrl: string;
  sources: string[];
  applyUrl: string;
};

function uniqueBoards(
  hits: { hit: ExtractedAtsHit; source: string }[],
): Map<string, AtsCandidate> {
  const byKey = new Map<string, AtsCandidate>();
  for (const { hit, source } of hits) {
    const key = tokenKey(hit.ats, hit.boardToken);
    const prev = byKey.get(key);
    if (prev) {
      if (!prev.sources.includes(source)) prev.sources.push(source);
      continue;
    }
    byKey.set(key, {
      name: hit.companyName,
      slug: slugify(hit.companyName),
      ats: hit.ats,
      boardToken: hit.boardToken,
      careersUrl: hit.careersUrl,
      sources: [source],
      applyUrl: hit.applyUrl,
    });
  }
  return byKey;
}

function disambiguateSlug(
  base: string,
  used: Set<string>,
  ats: string,
  token: string,
): string {
  let slug = base || slugify(token);
  if (!used.has(slug)) return slug;
  const withToken = `${base}-${slugify(token)}`.replace(/-+/g, "-");
  if (!used.has(withToken)) return withToken;
  const withAts = `${base}-${ats}`;
  if (!used.has(withAts)) return withAts;
  let i = 2;
  while (used.has(`${base}-${i}`)) i += 1;
  return `${base}-${i}`;
}

async function runAtsReport(opts: {
  asJson: boolean;
  write: boolean;
  dir: string;
}) {
  const registry = await loadRegistry(opts.dir);
  const allHits: { hit: ExtractedAtsHit; source: string }[] = [];
  let applyLinkCount = 0;
  let unsupportedApplyCount = 0;
  const unsupportedHosts: Record<string, number> = {};

  for (const src of ATS_SOURCES) {
    let md: string;
    try {
      md = await fetchText(src.url);
    } catch (err) {
      console.error(`warn: failed ${src.id}:`, err instanceof Error ? err.message : err);
      continue;
    }
    const extracted = extractAtsBoardsFromMarkdown(md);
    applyLinkCount += extracted.applyLinkCount;
    unsupportedApplyCount += extracted.unsupportedApplyCount;
    for (const [host, n] of Object.entries(extracted.unsupportedHosts)) {
      unsupportedHosts[host] = (unsupportedHosts[host] ?? 0) + n;
    }
    for (const hit of extracted.hits) {
      allHits.push({ hit, source: src.id });
    }
  }

  const boards = uniqueBoards(allHits);
  const covered: AtsCandidate[] = [];
  const inactive: (AtsCandidate & { registrySlug: string })[] = [];
  const tokenElsewhere: (AtsCandidate & { registrySlug: string; registryName: string })[] =
    [];
  const missing: AtsCandidate[] = [];

  for (const cand of boards.values()) {
    const key = tokenKey(cand.ats, cand.boardToken);
    const byTok = registry.byToken.get(key);
    if (byTok) {
      if (byTok.active) covered.push(cand);
      else inactive.push({ ...cand, registrySlug: byTok.slug });
      continue;
    }
    const byName = registry.byNorm.get(normalizeName(cand.name));
    if (byName) {
      // Same company name, different token — still a missing board token for us
      // unless we treat name match as covered. Prefer token match only for "covered".
      // If name exists under different token, list as missing (new board) but note.
      missing.push(cand);
      continue;
    }
    missing.push(cand);
  }

  // Probe missing candidates
  const probeOkList: AtsCandidate[] = [];
  const probeFailList: (AtsCandidate & { reason: string })[] = [];
  const CHUNK = 8;
  const missingArr = [...missing].sort((a, b) => a.name.localeCompare(b.name));
  for (let i = 0; i < missingArr.length; i += CHUNK) {
    const chunk = missingArr.slice(i, i + CHUNK);
    const results = await Promise.all(
      chunk.map(async (c) => {
        const ok = await probeOk(c.ats, c.boardToken);
        return { c, ok };
      }),
    );
    for (const r of results) {
      if (r.ok) probeOkList.push(r.c);
      else probeFailList.push({ ...r.c, reason: "probe failed" });
    }
  }

  let written: CompanyYaml[] = [];
  if (opts.write && probeOkList.length > 0) {
    const usedSlugs = new Set(registry.bySlug.keys());
    const techCPath = path.join(opts.dir, "tech-c.yaml");
    const additions: CompanyYaml[] = [];
    for (const c of probeOkList) {
      if (registry.byToken.has(tokenKey(c.ats, c.boardToken))) continue;
      const slug = disambiguateSlug(c.slug, usedSlugs, c.ats, c.boardToken);
      usedSlugs.add(slug);
      additions.push({
        name: c.name,
        slug,
        ats: c.ats,
        board_token: c.boardToken,
        careers_url: c.careersUrl,
        active: true,
      });
    }
    if (additions.length > 0) {
      const block = additions
        .map((e) => {
          const name =
            /[:#{}[\],&*?|>!%@`]/.test(e.name) || e.name.includes("'")
              ? JSON.stringify(e.name)
              : e.name;
          const token =
            /[:#{}[\],&*?|>!%@`]/.test(e.board_token) || e.board_token.includes(".")
              ? JSON.stringify(e.board_token)
              : e.board_token;
          return [
            `  - name: ${name}`,
            `    slug: ${e.slug}`,
            `    ats: ${e.ats}`,
            `    board_token: ${token}`,
            `    careers_url: ${e.careers_url}`,
          ].join("\n");
        })
        .join("\n");
      let raw = await fs.readFile(techCPath, "utf8");
      if (!raw.endsWith("\n")) raw += "\n";
      raw += `\n# Added from SimplifyJobs Apply-URL extraction (${new Date().toISOString().slice(0, 10)})\n`;
      raw += `${block}\n`;
      // Validate before committing write
      companiesFileSchema.parse(YAML.parse(raw));
      await fs.writeFile(techCPath, raw, "utf8");
      written = additions;
    }
  }

  const byAts: Record<string, number> = {};
  for (const c of boards.values()) {
    byAts[c.ats] = (byAts[c.ats] ?? 0) + 1;
  }

  const report = {
    generatedAt: new Date().toISOString(),
    mode: "ats" as const,
    registrySize: registry.bySlug.size,
    applyLinkCount,
    unsupportedApplyCount,
    unsupportedHosts: Object.fromEntries(
      Object.entries(unsupportedHosts).sort((a, b) => b[1] - a[1]),
    ),
    uniqueSupportedTokens: boards.size,
    byAts,
    coveredCount: covered.length,
    inactiveCount: inactive.length,
    missingCount: missing.length,
    probeOkCount: probeOkList.length,
    probeFailCount: probeFailList.length,
    writtenCount: written.length,
    covered: covered.map((c) => ({
      name: c.name,
      ats: c.ats,
      board_token: c.boardToken,
    })),
    inactive: inactive.map((c) => ({
      name: c.name,
      ats: c.ats,
      board_token: c.boardToken,
      registrySlug: c.registrySlug,
    })),
    missingProbeOk: probeOkList.map((c) => ({
      name: c.name,
      slug: c.slug,
      ats: c.ats,
      board_token: c.boardToken,
      careers_url: c.careersUrl,
      sources: c.sources,
    })),
    missingProbeFail: probeFailList.map((c) => ({
      name: c.name,
      ats: c.ats,
      board_token: c.boardToken,
      reason: c.reason,
    })),
    written: written.map((c) => ({
      name: c.name,
      slug: c.slug,
      ats: c.ats,
      board_token: c.board_token,
    })),
    tokenElsewhere,
  };

  if (opts.asJson) {
    console.log(JSON.stringify(report, null, 2));
    return report;
  }

  console.log("## SimplifyJobs ATS volume report\n");
  console.log(`Registry: ${report.registrySize} companies`);
  console.log(`Apply links: ${report.applyLinkCount}`);
  console.log(`  supported (parsed board): ${report.applyLinkCount - report.unsupportedApplyCount}`);
  console.log(`  unsupported hosts: ${report.unsupportedApplyCount}`);
  console.log(`Unique supported board tokens: ${report.uniqueSupportedTokens}`);
  console.log(`  by ATS: ${JSON.stringify(report.byAts)}`);
  console.log(`Already covered (active): ${report.coveredCount}`);
  console.log(`In registry but inactive: ${report.inactiveCount}`);
  console.log(`Missing from registry: ${report.missingCount}`);
  console.log(`  probe OK (addable): ${report.probeOkCount}`);
  console.log(`  probe failed: ${report.probeFailCount}`);
  if (opts.write) {
    console.log(`Written to tech-c.yaml: ${report.writtenCount}`);
  }

  console.log("\n### Top unsupported hosts\n");
  for (const [host, n] of Object.entries(report.unsupportedHosts).slice(0, 15)) {
    console.log(`- ${host}: ${n}`);
  }

  console.log("\n### Missing (probe OK)\n");
  for (const m of report.missingProbeOk) {
    console.log(
      `- ${m.name}  ats=${m.ats}  token=${m.board_token}  slug=${m.slug}  [${m.sources.join(", ")}]`,
    );
  }

  if (report.missingProbeFail.length) {
    console.log("\n### Missing (probe failed)\n");
    for (const m of report.missingProbeFail) {
      console.log(`- ${m.name}  ats=${m.ats}  token=${m.board_token}`);
    }
  }

  if (report.inactive.length) {
    console.log("\n### Inactive but listed (re-activate?)\n");
    for (const m of report.inactive) {
      console.log(
        `- ${m.name}  (${m.registrySlug})  ats=${m.ats}  token=${m.board_token}`,
      );
    }
  }

  if (written.length) {
    console.log("\n### Written\n");
    for (const w of written) {
      console.log(`- ${w.name} (${w.slug}) ${w.ats}/${w.board_token}`);
    }
  }

  console.log(
    "\nNext: pnpm validate-companies && pnpm validate-tokens -- ../../data/companies/tech-c.yaml",
  );
  console.log("Then: pnpm sync-companies && ingest (when ready).");
  console.log("YC/a16z deferred until after reviewing this volume.\n");

  return report;
}

async function runNameReport(opts: { asJson: boolean; topN: number; dir: string }) {
  const registry = await loadRegistry(opts.dir);

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
    top: opts.topN,
    missing: missing.slice(0, opts.topN),
    inactiveButListed: inactiveHits.slice(0, Math.min(opts.topN, 100)),
  };

  if (opts.asJson) {
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

async function main() {
  const asJson = process.argv.includes("--json");
  const atsMode = process.argv.includes("--ats") || process.argv.includes("--from-apply");
  const write = process.argv.includes("--write");
  const topArg = process.argv.find((a) => a.startsWith("--top="));
  const topN = topArg ? Math.max(1, Number(topArg.split("=")[1]) || 60) : 60;
  const dir = defaultCompaniesDir();

  if (atsMode) {
    await runAtsReport({ asJson, write, dir });
    return;
  }
  await runNameReport({ asJson, topN, dir });
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
