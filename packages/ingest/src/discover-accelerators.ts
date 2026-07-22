/**
 * Discover public ATS boards for YC / a16z portfolio companies.
 *
 * YC: https://yc-oss.github.io/api/companies/hiring.json (tech-filtered)
 * a16z: curated scrape of https://a16z.com/portfolio-companies/ company names + websites
 *
 * Usage:
 *   tsx src/discover-accelerators.ts --yc [--write] [--limit=200]
 *   tsx src/discover-accelerators.ts --a16z [--write] [--limit=200]
 *   tsx src/discover-accelerators.ts --yc --a16z --write
 */
import fs from "node:fs/promises";
import path from "node:path";
import YAML from "yaml";
import type { AtsName } from "./ats.js";
import { ALL_ATS, probeAtsBoard } from "./probe-url.js";
import { companiesFileSchema, type CompanyYaml } from "./schema.js";
import { defaultCompaniesDir } from "./sync-companies.js";
import { tokenKey } from "./parse-ats-url.js";
import { slugify } from "./gap-report.js";

const UA = "OpenIntern/0.1 (+https://github.com/dnexdev/openintern; discover-accelerators)";

const YC_HIRING_URL = "https://yc-oss.github.io/api/companies/hiring.json";

const TECH_INDUSTRY =
  /^(B2B|Consumer|Fintech|Healthcare|Education|Industrials|Unspecified)/i;
const TECH_TAGS =
  /ai|artificial intelligence|machine learning|developer tools|saas|fintech|infra|security|crypto|devtools|api|data|robotics|hardware|semiconductor|climate|biotech|health tech|open source/i;

type YcCompany = {
  name: string;
  slug: string;
  website?: string | null;
  industry?: string | null;
  tags?: string[];
  status?: string;
  batch?: string;
  isHiring?: boolean;
};

function isTechYc(c: YcCompany): boolean {
  if (c.status && /inactive|dead/i.test(c.status)) return false;
  if (c.industry && TECH_INDUSTRY.test(c.industry)) return true;
  if ((c.tags ?? []).some((t) => TECH_TAGS.test(t))) return true;
  // Default include hiring companies without industry — still tech-leaning YC set
  return !c.industry;
}

function tokenCandidates(slug: string, name: string, website?: string | null): string[] {
  const compact = name.toLowerCase().replace(/[^a-z0-9]/g, "");
  const dashed = slugify(name) || slug;
  const nospace = dashed.replace(/-/g, "");
  const first = dashed.split("-")[0] ?? dashed;
  const set = new Set<string>([dashed, nospace, compact, first, slug]);
  if (website) {
    try {
      const host = new URL(website).hostname.replace(/^www\./, "");
      const base = host.split(".")[0];
      if (base && base.length >= 2) set.add(base);
    } catch {
      /* ignore */
    }
  }
  return [...set].filter((t) => t.length >= 2 && t.length <= 48).slice(0, 4);
}

type RegistryMaps = {
  bySlug: Set<string>;
  byToken: Set<string>;
  byNormName: Set<string>;
};

function normalizeName(name: string): string {
  return name
    .toLowerCase()
    .normalize("NFKD")
    .replace(/&/g, " and ")
    .replace(/[^a-z0-9]+/g, "")
    .trim();
}

async function loadRegistry(dir: string): Promise<RegistryMaps> {
  const bySlug = new Set<string>();
  const byToken = new Set<string>();
  const byNormName = new Set<string>();
  const entries = await fs.readdir(dir);
  for (const file of entries.filter((f) => f.endsWith(".yaml") || f.endsWith(".yml"))) {
    const raw = await fs.readFile(path.join(dir, file), "utf8");
    const data = companiesFileSchema.parse(YAML.parse(raw));
    for (const c of data.companies) {
      bySlug.add(c.slug);
      byToken.add(tokenKey(c.ats, c.board_token));
      byNormName.add(normalizeName(c.name));
    }
  }
  return { bySlug, byToken, byNormName };
}

type Found = {
  name: string;
  slug: string;
  ats: AtsName;
  board_token: string;
  website_url?: string;
  source: string;
};

async function probeCompany(
  name: string,
  slugHint: string,
  website: string | null | undefined,
  registry: RegistryMaps,
  source: string,
): Promise<Found | null> {
  if (registry.byNormName.has(normalizeName(name))) return null;
  const tokens = tokenCandidates(slugHint, name, website);
  // Prefer common startup ATSes first
  const atsOrder: AtsName[] = [
    "ashby",
    "greenhouse",
    "lever",
    "rippling",
    "workable",
    "smartrecruiters",
    "recruitee",
    "bamboohr",
  ];
  for (const ats of atsOrder) {
    if (!ALL_ATS.includes(ats)) continue;
    for (const token of tokens) {
      const key = tokenKey(ats, token);
      if (registry.byToken.has(key)) continue;
      const result = await probeAtsBoard(ats, token, UA);
      if (!result.ok) continue;
      // Prefer boards that actually have jobs; allow empty (count 0) only if HTTP OK
      const slugBase = slugify(name) || slugHint || token.toLowerCase();
      let slug = slugBase;
      let i = 2;
      while (registry.bySlug.has(slug)) {
        slug = `${slugBase}-${i++}`;
      }
      return {
        name,
        slug,
        ats,
        board_token: token,
        website_url: website ?? undefined,
        source,
      };
    }
  }
  return null;
}

async function fetchYcHiring(limit: number): Promise<YcCompany[]> {
  const res = await fetch(YC_HIRING_URL, {
    headers: { "User-Agent": UA, Accept: "application/json" },
  });
  if (!res.ok) throw new Error(`YC hiring.json HTTP ${res.status}`);
  const all = (await res.json()) as YcCompany[];
  const tech = all.filter(isTechYc);
  return tech.slice(0, limit);
}

/** Best-effort parse of a16z portfolio directory page for company names + URLs. */
async function fetchA16zPortfolio(limit: number): Promise<{ name: string; website?: string }[]> {
  const urls = [
    "https://a16z.com/portfolio-companies/",
    "https://a16z.com/portfolio/",
  ];
  const found = new Map<string, { name: string; website?: string }>();
  for (const url of urls) {
    try {
      const res = await fetch(url, {
        headers: {
          "User-Agent": UA,
          Accept: "text/html,application/xhtml+xml",
        },
      });
      if (!res.ok) continue;
      const html = await res.text();
      // Portfolio cards often link out: <a href="https://company.com">Name</a>
      for (const m of html.matchAll(
        /<a[^>]+href=["'](https?:\/\/(?!(?:www\.)?a16z\.com)[^"']+)["'][^>]*>([^<]{2,80})<\/a>/gi,
      )) {
        const website = m[1]!;
        const name = m[2]!.replace(/\s+/g, " ").trim();
        if (!/[a-zA-Z]/.test(name)) continue;
        if (/follow|subscribe|twitter|linkedin|careers|privacy|terms/i.test(name)) continue;
        if (/linkedin\.com|twitter\.com|x\.com|facebook\.com|youtube\.com/i.test(website)) {
          continue;
        }
        const key = normalizeName(name);
        if (!found.has(key)) found.set(key, { name, website });
      }
      // JSON-LD or embedded portfolio blobs
      for (const m of html.matchAll(/"name"\s*:\s*"([^"]{2,80})"\s*,\s*"url"\s*:\s*"(https?:\/\/[^"]+)"/g)) {
        const name = m[1]!;
        const website = m[2]!;
        if (/a16z\.com/i.test(website)) continue;
        const key = normalizeName(name);
        if (!found.has(key)) found.set(key, { name, website });
      }
    } catch {
      /* try next */
    }
  }

  // Prefer curated seed when the page is JS-rendered / sparse
  const seed = [
    { name: "OpenAI", website: "https://openai.com" },
    { name: "Databricks", website: "https://databricks.com" },
    { name: "Stripe", website: "https://stripe.com" },
    { name: "Ramp", website: "https://ramp.com" },
    { name: "Anduril", website: "https://www.anduril.com" },
    { name: "Figma", website: "https://www.figma.com" },
    { name: "Notion", website: "https://www.notion.so" },
    { name: "Roblox", website: "https://www.roblox.com" },
    { name: "Coinbase", website: "https://www.coinbase.com" },
    { name: "Instacart", website: "https://www.instacart.com" },
    { name: "Pinterest", website: "https://www.pinterest.com" },
    { name: "Airbnb", website: "https://www.airbnb.com" },
    { name: "Okta", website: "https://www.okta.com" },
    { name: "Snowflake", website: "https://www.snowflake.com" },
    { name: "MongoDB", website: "https://www.mongodb.com" },
    { name: "Samsara", website: "https://www.samsara.com" },
    { name: "Robinhood", website: "https://robinhood.com" },
    { name: "Affirm", website: "https://www.affirm.com" },
    { name: "Chime", website: "https://www.chime.com" },
    { name: "Scale AI", website: "https://scale.com" },
    { name: "Hugging Face", website: "https://huggingface.co" },
    { name: "Character.AI", website: "https://character.ai" },
    { name: "Anysphere", website: "https://cursor.com" },
    { name: "Retool", website: "https://retool.com" },
    { name: "Vercel", website: "https://vercel.com" },
    { name: "Linear", website: "https://linear.app" },
    { name: "Mercury", website: "https://mercury.com" },
    { name: "Rippling", website: "https://www.rippling.com" },
    { name: "Deel", website: "https://www.deel.com" },
    { name: "Brex", website: "https://www.brex.com" },
    { name: "Plaid", website: "https://plaid.com" },
    { name: "Faire", website: "https://www.faire.com" },
    { name: "Flexport", website: "https://www.flexport.com" },
    { name: "Navan", website: "https://navan.com" },
    { name: "Asana", website: "https://asana.com" },
    { name: "Box", website: "https://www.box.com" },
    { name: "Airtable", website: "https://www.airtable.com" },
    { name: "Webflow", website: "https://webflow.com" },
    { name: "Fivetran", website: "https://www.fivetran.com" },
    { name: "dbt Labs", website: "https://www.getdbt.com" },
    { name: "Harness", website: "https://www.harness.io" },
    { name: "Snyk", website: "https://snyk.io" },
    { name: "Wiz", website: "https://www.wiz.io" },
    { name: "Cribl", website: "https://cribl.io" },
    { name: "Gong", website: "https://www.gong.io" },
    { name: "Ironclad", website: "https://ironcladapp.com" },
    { name: "Dialpad", website: "https://www.dialpad.com" },
    { name: "Opendoor", website: "https://www.opendoor.com" },
    { name: "NerdWallet", website: "https://www.nerdwallet.com" },
    { name: "Reddit", website: "https://www.reddit.com" },
  ];
  for (const s of seed) {
    const key = normalizeName(s.name);
    if (!found.has(key)) found.set(key, s);
  }

  return [...found.values()]
    .filter((c) => !/^https?:/i.test(c.name) && /[a-zA-Z]/.test(c.name))
    .slice(0, limit);
}

function yamlEscape(value: string): string {
  if (/[:#{}[\],&*?|>!%@`]/.test(value) || value.includes("'") || value.includes(".")) {
    return JSON.stringify(value);
  }
  return value;
}

async function appendCompanies(dir: string, additions: Found[], marker: string) {
  if (additions.length === 0) return;
  const outPath = path.join(dir, "tech-accelerators.yaml");
  let raw = "";
  try {
    raw = await fs.readFile(outPath, "utf8");
  } catch {
    raw = "";
  }
  const used = new Set<string>();
  if (raw.trim()) {
    try {
      const existing = companiesFileSchema.parse(YAML.parse(raw));
      for (const c of existing.companies) used.add(c.slug);
    } catch {
      /* start fresh if corrupt/empty */
      raw = "";
    }
  }
  const entries: CompanyYaml[] = [];
  for (const f of additions) {
    let slug = f.slug;
    let i = 2;
    while (used.has(slug)) slug = `${f.slug}-${i++}`;
    used.add(slug);
    const entry: CompanyYaml = {
      name: f.name,
      slug,
      ats: f.ats,
      board_token: f.board_token,
      active: true,
      ...(f.website_url ? { website_url: f.website_url } : {}),
    };
    entries.push(entry);
  }
  const block = entries
    .map((e) => {
      const lines = [
        `  - name: ${yamlEscape(e.name)}`,
        `    slug: ${e.slug}`,
        `    ats: ${e.ats}`,
        `    board_token: ${yamlEscape(e.board_token)}`,
      ];
      if (e.website_url) lines.push(`    website_url: ${e.website_url}`);
      return lines.join("\n");
    })
    .join("\n");
  if (!raw.trim()) {
    raw = `# YC / a16z accelerator ATS discoveries (probe-OK only)\ncompanies:\n${block}\n`;
  } else {
    if (!raw.endsWith("\n")) raw += "\n";
    raw += `\n# ${marker}\n${block}\n`;
  }
  companiesFileSchema.parse(YAML.parse(raw));
  await fs.writeFile(outPath, raw, "utf8");
}

async function main() {
  const args = process.argv.slice(2);
  const doYc = args.includes("--yc");
  const doA16z = args.includes("--a16z");
  const write = args.includes("--write");
  const limitArg = args.find((a) => a.startsWith("--limit="));
  const limit = limitArg ? Math.max(1, Number(limitArg.split("=")[1]) || 200) : 200;

  if (!doYc && !doA16z) {
    console.error("Usage: discover-accelerators.ts --yc | --a16z [--write] [--limit=N]");
    process.exit(1);
  }

  const dir = defaultCompaniesDir();
  const registry = await loadRegistry(dir);
  const found: Found[] = [];
  let ycTried = 0;
  let a16zTried = 0;

  if (doYc) {
    console.log(`Fetching YC hiring.json (tech filter, limit ${limit})…`);
    const companies = await fetchYcHiring(limit);
    console.log(`YC candidates: ${companies.length}`);
    for (const c of companies) {
      ycTried += 1;
      if (ycTried % 25 === 0) console.log(`  probed ${ycTried}/${companies.length}…`);
      const hit = await probeCompany(
        c.name,
        c.slug || slugify(c.name),
        c.website,
        registry,
        `yc:${c.batch ?? "hiring"}`,
      );
      if (hit) {
        found.push(hit);
        registry.bySlug.add(hit.slug);
        registry.byToken.add(tokenKey(hit.ats, hit.board_token));
        registry.byNormName.add(normalizeName(hit.name));
        console.log(`  + ${hit.name} → ${hit.ats}/${hit.board_token}`);
      }
    }
  }

  if (doA16z) {
    console.log(`Fetching a16z portfolio (limit ${limit})…`);
    const companies = await fetchA16zPortfolio(limit);
    console.log(`a16z candidates: ${companies.length}`);
    for (const c of companies) {
      a16zTried += 1;
      const hit = await probeCompany(
        c.name,
        slugify(c.name),
        c.website,
        registry,
        "a16z-portfolio",
      );
      if (hit) {
        found.push(hit);
        registry.bySlug.add(hit.slug);
        registry.byToken.add(tokenKey(hit.ats, hit.board_token));
        registry.byNormName.add(normalizeName(hit.name));
        console.log(`  + ${hit.name} → ${hit.ats}/${hit.board_token}`);
      }
    }
  }

  console.log("\n## Summary");
  console.log(`YC probed: ${ycTried}`);
  console.log(`a16z probed: ${a16zTried}`);
  console.log(`New boards found: ${found.length}`);
  for (const f of found) {
    console.log(`- ${f.name} (${f.slug}) ${f.ats}/${f.board_token} [${f.source}]`);
  }

  if (write) {
    await appendCompanies(
      dir,
      found,
      `Added from accelerator ATS discovery (${new Date().toISOString().slice(0, 10)})`,
    );
    console.log(`\nWrote ${found.length} companies to data/companies/tech-accelerators.yaml`);
  } else {
    console.log("\nDry run — pass --write to append probe-OK companies.");
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
