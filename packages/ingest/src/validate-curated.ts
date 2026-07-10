import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import YAML from "yaml";
import { tier1CuratedSchema } from "./schema.js";
import { defaultCompaniesDir, loadCompanyYamlFiles } from "./sync-companies.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export function defaultTier1Path(): string {
  return path.resolve(__dirname, "../../../data/curated/tier-1.yaml");
}

export async function loadTier1Curated(filePath = defaultTier1Path()) {
  const raw = await fs.readFile(filePath, "utf8");
  return tier1CuratedSchema.parse(YAML.parse(raw));
}

export async function validateTier1Curated(opts?: {
  tier1Path?: string;
  companiesDir?: string;
}) {
  const tier1Path = opts?.tier1Path ?? defaultTier1Path();
  const companiesDir = opts?.companiesDir ?? defaultCompaniesDir();
  const { slugs } = await loadTier1Curated(tier1Path);
  const companies = await loadCompanyYamlFiles(companiesDir);
  const bySlug = new Map(companies.map((c) => [c.slug, c]));

  const seen = new Set<string>();
  const errors: string[] = [];
  const warnings: string[] = [];

  for (const slug of slugs) {
    if (seen.has(slug)) {
      errors.push(`duplicate slug: ${slug}`);
      continue;
    }
    seen.add(slug);

    const company = bySlug.get(slug);
    if (!company) {
      errors.push(`unknown slug (not in data/companies): ${slug}`);
      continue;
    }
    if (company.active === false) {
      warnings.push(`${slug} is active: false — no jobs will be highlighted`);
    }
  }

  return { errors, warnings, count: slugs.length };
}

async function main() {
  const { errors, warnings, count } = await validateTier1Curated();

  for (const w of warnings) {
    console.warn(`warn: ${w}`);
  }

  if (errors.length > 0) {
    for (const e of errors) {
      console.error(`error: ${e}`);
    }
    process.exit(1);
  }

  console.log(`Tier 1 curated list OK (${count} slugs).`);
}

const isMain =
  process.argv[1] &&
  (process.argv[1].endsWith("validate-curated.ts") ||
    process.argv[1].endsWith("validate-curated.js"));

if (isMain) {
  main().catch((err) => {
    console.error(err);
    process.exit(1);
  });
}
