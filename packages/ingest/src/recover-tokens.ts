/**
 * Probe inactive / broken company tokens against all supported ATSes
 * with common board_token spellings. Prints YAML-ready suggestions.
 *
 * Usage:
 *   tsx src/recover-tokens.ts
 *   tsx src/recover-tokens.ts --limit=40
 *   tsx src/recover-tokens.ts --slug=palantir,netflix
 *   tsx src/recover-tokens.ts --write
 */
import fs from "node:fs/promises";
import path from "node:path";
import YAML from "yaml";
import type { AtsName } from "./ats.js";
import { ALL_ATS, countJobsFromProbeBody, probeUrl } from "./probe-url.js";
import { companiesFileSchema } from "./schema.js";
import { defaultCompaniesDir } from "./sync-companies.js";

const UA = "OpenIntern/0.1 (+https://github.com/dnexdev/openintern; recover-tokens)";

type CompanyRow = {
  name: string;
  slug: string;
  ats: string;
  board_token: string;
  active: boolean;
  website_url?: string;
  careers_url?: string;
  file: string;
};

function titleCaseToken(slug: string): string {
  return slug
    .split("-")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join("");
}

function tokenCandidates(slug: string, name: string, current: string, ats: AtsName): string[] {
  const compact = name.toLowerCase().replace(/[^a-z0-9]/g, "");
  const dashed = slug;
  const nospace = slug.replace(/-/g, "");
  const first = slug.split("-")[0] ?? slug;
  const title = titleCaseToken(slug);
  const set = new Set<string>([
    current,
    dashed,
    nospace,
    compact,
    first,
    title,
    `${nospace}careers`,
    `${first}careers`,
  ]);
  if (ats === "smartrecruiters") {
    set.add(title);
    set.add(name.split(/\s+/)[0] ?? title);
    for (const part of name.split(/\s+/)) {
      if (part.length >= 2) set.add(part);
    }
  }
  if (ats === "recruitee" || ats === "bamboohr") {
    return [...set].filter((t) => /^[a-z0-9-]+$/i.test(t) && t.length >= 2);
  }
  return [...set].filter((t) => t.length >= 2);
}

async function probe(ats: AtsName, token: string): Promise<{ ok: boolean; count?: number }> {
  const url = probeUrl(ats, token);
  try {
    const res = await fetch(url, {
      headers: { Accept: "application/json", "User-Agent": UA },
    });
    if (!res.ok) return { ok: false };
    const data = (await res.json()) as unknown;
    return { ok: true, count: countJobsFromProbeBody(ats, data) };
  } catch {
    return { ok: false };
  }
}

async function loadCompanies(dir: string): Promise<CompanyRow[]> {
  const entries = await fs.readdir(dir);
  const all: CompanyRow[] = [];
  for (const file of entries.filter((f) => f.endsWith(".yaml") || f.endsWith(".yml"))) {
    const raw = await fs.readFile(path.join(dir, file), "utf8");
    const data = companiesFileSchema.parse(YAML.parse(raw));
    for (const c of data.companies) {
      all.push({
        name: c.name,
        slug: c.slug,
        ats: c.ats,
        board_token: c.board_token,
        active: c.active ?? true,
        website_url: c.website_url,
        careers_url: c.careers_url,
        file,
      });
    }
  }
  return all;
}

/** High-value inactive / often-wrong brands to try first. */
const PRIORITY_SLUGS = [
  "palantir",
  "netflix",
  "adobe",
  "atlassian",
  "doordash",
  "salesforce",
  "intuit",
  "zoom",
  "slack",
  "shopify",
  "nvidia",
  "uber",
  "lyft",
  "airbnb",
  "dropbox",
  "twitch",
  "robinhood",
  "coinbase",
  "databricks",
  "snowflake",
  "mongodb",
  "elastic",
  "gitlab",
  "hashicorp",
  "cloudflare",
  "okta",
  "twilio",
  "hubspot",
  "asana",
  "notion",
  "figma",
  "canva",
  "docusign",
  "zendesk",
  "splunk",
  "crowdstrike",
  "palo-alto-networks",
  "two-sigma",
  "hrt",
  "citadel",
  "jane-street",
  "optiver",
  "drw",
  "akuna",
  "tesla",
  "spacex",
  "anduril",
  "rivian",
  "sony",
  "samsung",
  "intel",
  "amd",
  "qualcomm",
  "broadcom",
  "cisco",
  "oracle",
  "ibm",
  "sap",
  "workday",
  "servicenow",
  "autodesk",
  "ea",
  "activision",
  "riot-games",
  "roblox",
  "unity",
  "epic-games",
];

type RecoverySuggestion = {
  slug: string;
  name: string;
  file: string;
  from: { ats: string; token: string; active: boolean };
  to: { ats: AtsName; token: string; jobCount: number };
  website_url?: string;
  careers_url?: string;
};

async function findRecovery(
  c: CompanyRow,
): Promise<{ ats: AtsName; token: string; count: number } | null> {
  const atsOrder: AtsName[] = [
    c.ats as AtsName,
    ...ALL_ATS.filter((a) => a !== c.ats),
  ].filter((a) => ALL_ATS.includes(a));

  for (const ats of atsOrder) {
    for (const token of tokenCandidates(c.slug, c.name, c.board_token, ats)) {
      const result = await probe(ats, token);
      if (!result.ok) continue;
      if (c.active && ats === c.ats && token === c.board_token) continue;
      return { ats, token, count: result.count ?? 0 };
    }
  }
  return null;
}

async function writeSuggestion(dir: string, suggestion: RecoverySuggestion): Promise<void> {
  const filePath = path.join(dir, suggestion.file);
  const raw = await fs.readFile(filePath, "utf8");
  const parsed = companiesFileSchema.parse(YAML.parse(raw));
  let updated = false;
  for (const company of parsed.companies) {
    if (company.slug !== suggestion.slug) continue;
    company.ats = suggestion.to.ats;
    company.board_token = suggestion.to.token;
    company.active = true;
    updated = true;
  }
  if (!updated) return;
  await fs.writeFile(filePath, YAML.stringify(parsed));
}

async function main() {
  const args = process.argv.slice(2).filter((a) => a !== "--");
  const limitArg = args.find((a) => a.startsWith("--limit="));
  const limit = limitArg ? Number(limitArg.split("=")[1]) : 40;
  const slugArg = args.find((a) => a.startsWith("--slug="));
  const onlySlugs = slugArg
    ? new Set(slugArg.split("=")[1]!.split(",").map((s) => s.trim()))
    : null;
  const shouldWrite = args.includes("--write");

  const dir = defaultCompaniesDir();
  const companies = await loadCompanies(dir);

  let targets: CompanyRow[];
  if (onlySlugs) {
    targets = companies.filter((c) => onlySlugs.has(c.slug));
  } else {
    const inactive = companies.filter((c) => !c.active);
    const priority = PRIORITY_SLUGS.map((s) => companies.find((c) => c.slug === s)).filter(
      Boolean,
    ) as CompanyRow[];
    const rest = inactive.filter((c) => !PRIORITY_SLUGS.includes(c.slug));
    targets = [...priority, ...rest].slice(0, limit);
  }

  const suggestions: RecoverySuggestion[] = [];

  for (const c of targets) {
    process.stderr.write(`probing ${c.slug}...\n`);
    const found = await findRecovery(c);
    if (!found) continue;
    if (found.ats === c.ats && found.token === c.board_token && c.active) continue;

    suggestions.push({
      slug: c.slug,
      name: c.name,
      file: c.file,
      from: { ats: c.ats, token: c.board_token, active: c.active },
      to: { ats: found.ats, token: found.token, jobCount: found.count },
      website_url: c.website_url,
      careers_url: c.careers_url,
    });
    console.log(
      `RECOVER ${c.slug}: ${c.ats}/${c.board_token} → ${found.ats}/${found.token} (${found.count} jobs)`,
    );
  }

  const useful = suggestions.filter((s) => s.to.jobCount > 0);
  console.log(
    `\n# ${useful.length} recovery suggestion(s) with jobs (of ${suggestions.length} probes)\n`,
  );
  for (const s of useful) {
    console.log(`# ${s.name} (${s.slug}) — was ${s.from.ats}/${s.from.token}`);
    console.log(`  - name: ${s.name}`);
    console.log(`    slug: ${s.slug}`);
    console.log(`    ats: ${s.to.ats}`);
    console.log(`    board_token: ${s.to.token}`);
    console.log(`    active: true`);
    if (s.website_url) console.log(`    website_url: ${s.website_url}`);
    else console.log(`    website_url: https://www.${s.slug.replace(/-/g, "")}.com`);
    console.log("");
  }

  if (shouldWrite && useful.length > 0) {
    for (const s of useful) {
      await writeSuggestion(dir, s);
      console.log(`wrote ${s.slug} → ${s.file}`);
    }
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
