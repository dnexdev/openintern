import fs from "node:fs/promises";
import path from "node:path";
import YAML from "yaml";
import { companiesFileSchema, type CompanyYaml } from "./schema.js";
import { defaultCompaniesDir } from "./sync-companies.js";

type SourcedCompany = CompanyYaml & { file: string };

export async function loadSourcedCompanies(
  dir = defaultCompaniesDir(),
): Promise<SourcedCompany[]> {
  const entries = (await fs.readdir(dir))
    .filter((file) => file.endsWith(".yaml") || file.endsWith(".yml"))
    .sort();
  const companies: SourcedCompany[] = [];

  for (const file of entries) {
    const raw = await fs.readFile(path.join(dir, file), "utf8");
    const parsed = companiesFileSchema.parse(YAML.parse(raw));
    companies.push(...parsed.companies.map((company) => ({ ...company, file })));
  }

  return companies;
}

export async function validateCompanies(dir = defaultCompaniesDir()) {
  const companies = await loadSourcedCompanies(dir);
  const errors: string[] = [];
  const warnings: string[] = [];
  const slugSources = new Map<string, string>();
  const tokenSources = new Map<string, SourcedCompany>();

  for (const company of companies) {
    const priorFile = slugSources.get(company.slug);
    if (priorFile) {
      errors.push(`duplicate slug "${company.slug}" in ${priorFile} and ${company.file}`);
    } else {
      slugSources.set(company.slug, company.file);
    }

    const tokenKey = `${company.ats}:${company.board_token}`;
    const priorToken = tokenSources.get(tokenKey);
    if (priorToken && priorToken.slug !== company.slug) {
      warnings.push(
        `duplicate ATS token "${tokenKey}" for ${priorToken.slug} (${priorToken.file}) and ${company.slug} (${company.file})`,
      );
    } else {
      tokenSources.set(tokenKey, company);
    }

    if (!company.website_url) {
      warnings.push(`missing website_url for ${company.slug} (${company.file})`);
    }
  }

  return { count: companies.length, errors, warnings };
}

async function main() {
  const result = await validateCompanies();
  const missingWebsites = result.warnings.filter((warning) =>
    warning.startsWith("missing website_url"),
  );
  const otherWarnings = result.warnings.filter(
    (warning) => !warning.startsWith("missing website_url"),
  );
  for (const warning of otherWarnings) console.warn(`warn: ${warning}`);
  if (missingWebsites.length > 0) {
    console.warn(
      `warn: ${missingWebsites.length} companies are missing website_url (logos may use fallbacks)`,
    );
  }
  for (const error of result.errors) console.error(`error: ${error}`);

  if (result.errors.length > 0) process.exit(1);
  console.log(`Company registry OK (${result.count} entries).`);
}

const isMain =
  process.argv[1]?.endsWith("validate-companies.ts") ||
  process.argv[1]?.endsWith("validate-companies.js");

if (isMain) {
  main().catch((error) => {
    console.error(error);
    process.exit(1);
  });
}
