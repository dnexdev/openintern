/**
 * Probe inactive / broken company tokens against Greenhouse, Lever, Ashby
 * with common board_token spellings. Prints YAML-ready suggestions.
 *
 * Usage:
 *   tsx src/recover-tokens.ts
 *   tsx src/recover-tokens.ts --limit 40
 *   tsx src/recover-tokens.ts --slug palantir,netflix
 */
import fs from "node:fs/promises";
import path from "node:path";
import YAML from "yaml";
import { companiesFileSchema } from "./schema.js";
import { defaultCompaniesDir } from "./sync-companies.js";

const UA = "OpenIntern/0.1 (+https://github.com/dnexdev/openintern; recover-tokens)";

type Ats = "greenhouse" | "lever" | "ashby";

type CompanyRow = {
  name: string;
  slug: string;
  ats: string;
  board_token: string;
  active: boolean;
  website_url?: string;
  file: string;
};

function probeUrl(ats: Ats, token: string): string {
  switch (ats) {
    case "greenhouse":
      return `https://boards-api.greenhouse.io/v1/boards/${encodeURIComponent(token)}/jobs`;
    case "lever":
      return `https://api.lever.co/v0/postings/${encodeURIComponent(token)}?mode=json`;
    case "ashby":
      return `https://api.ashbyhq.com/posting-api/job-board/${encodeURIComponent(token)}`;
  }
}

function tokenCandidates(slug: string, name: string, current: string): string[] {
  const compact = name.toLowerCase().replace(/[^a-z0-9]/g, "");
  const dashed = slug;
  const nospace = slug.replace(/-/g, "");
  const first = slug.split("-")[0] ?? slug;
  const set = new Set<string>([
    current,
    dashed,
    nospace,
    compact,
    first,
    `${nospace}careers`,
    `${first}careers`,
  ]);
  return [...set].filter((t) => t.length >= 2);
}

async function probe(ats: Ats, token: string): Promise<{ ok: boolean; count?: number }> {
  const url = probeUrl(ats, token);
  try {
    const res = await fetch(url, {
      headers: { Accept: "application/json", "User-Agent": UA },
    });
    if (!res.ok) return { ok: false };
    const data = (await res.json()) as unknown;
    let count = 0;
    if (Array.isArray(data)) count = data.length;
    else if (data && typeof data === "object") {
      const o = data as Record<string, unknown>;
      if (Array.isArray(o.jobs)) count = o.jobs.length;
      else if (Array.isArray(o.results)) count = o.results.length;
      else count = 1;
    }
    return { ok: true, count };
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

async function main() {
  const args = process.argv.slice(2).filter((a) => a !== "--");
  const limitArg = args.find((a) => a.startsWith("--limit="));
  const limit = limitArg ? Number(limitArg.split("=")[1]) : 40;
  const slugArg = args.find((a) => a.startsWith("--slug="));
  const onlySlugs = slugArg
    ? new Set(slugArg.split("=")[1]!.split(",").map((s) => s.trim()))
    : null;

  const dir = defaultCompaniesDir();
  const companies = await loadCompanies(dir);

  let targets = companies.filter((c) => !c.active || onlySlugs?.has(c.slug));
  if (onlySlugs) {
    targets = companies.filter((c) => onlySlugs.has(c.slug));
  } else {
    const priority = PRIORITY_SLUGS.map((s) => companies.find((c) => c.slug === s)).filter(
      Boolean,
    ) as CompanyRow[];
    const rest = targets.filter((c) => !PRIORITY_SLUGS.includes(c.slug));
    targets = [...priority, ...rest].slice(0, limit);
  }

  const suggestions: {
    slug: string;
    name: string;
    from: { ats: string; token: string; active: boolean };
    to: { ats: Ats; token: string; jobCount: number };
    website_url?: string;
  }[] = [];

  const ATS_ORDER: Ats[] = ["ashby", "greenhouse", "lever"];

  for (const c of targets) {
    process.stderr.write(`probing ${c.slug}...\n`);
    let found: { ats: Ats; token: string; count: number } | null = null;

    outer: for (const ats of ATS_ORDER) {
      for (const token of tokenCandidates(c.slug, c.name, c.board_token)) {
        // Skip identical current config if already active+ok — still try for inactive
        const result = await probe(ats, token);
        if (result.ok) {
          // Prefer a different working board than a known-dead current
          if (c.active && ats === c.ats && token === c.board_token) continue;
          found = { ats, token, count: result.count ?? 0 };
          break outer;
        }
      }
    }

    if (found) {
      if (found.ats === c.ats && found.token === c.board_token && c.active) continue;
      suggestions.push({
        slug: c.slug,
        name: c.name,
        from: { ats: c.ats, token: c.board_token, active: c.active },
        to: { ats: found.ats, token: found.token, jobCount: found.count },
        website_url: c.website_url,
      });
      console.log(
        `RECOVER ${c.slug}: ${c.ats}/${c.board_token} → ${found.ats}/${found.token} (${found.count} jobs)`,
      );
    }
  }

  // Prefer boards that actually have postings; 0-job boards are weak recoveries.
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
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
