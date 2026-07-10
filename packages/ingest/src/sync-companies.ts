import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import YAML from "yaml";
import { eq } from "drizzle-orm";
import { companies, type Db } from "@openintern/db";
import { companiesFileSchema } from "./schema.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export function defaultCompaniesDir(): string {
  return path.resolve(__dirname, "../../../data/companies");
}

export async function loadCompanyYamlFiles(dir = defaultCompaniesDir()) {
  const entries = await fs.readdir(dir);
  // Lexicographic order so later filenames win on duplicate slugs (ats-new → tech-*).
  const yamlFiles = entries
    .filter((f) => f.endsWith(".yaml") || f.endsWith(".yml"))
    .sort();
  const all = [];
  for (const file of yamlFiles) {
    const raw = await fs.readFile(path.join(dir, file), "utf8");
    const parsed = YAML.parse(raw);
    const data = companiesFileSchema.parse(parsed);
    all.push(...data.companies);
  }
  return all;
}

export async function syncCompaniesFromYaml(db: Db, dir = defaultCompaniesDir()) {
  const list = await loadCompanyYamlFiles(dir);
  // Later files win on duplicate slugs (e.g. tech-b overrides ats-new).
  const bySlug = new Map<string, (typeof list)[number]>();
  for (const c of list) bySlug.set(c.slug, c);

  let upserted = 0;
  for (const c of bySlug.values()) {
    const existing = await db.query.companies.findFirst({
      where: eq(companies.slug, c.slug),
    });
    if (existing) {
      await db
        .update(companies)
        .set({
          name: c.name,
          ats: c.ats,
          boardToken: c.board_token,
          careersUrl: c.careers_url ?? null,
          websiteUrl: c.website_url ?? null,
          active: c.active ?? true,
          updatedAt: new Date(),
        })
        .where(eq(companies.id, existing.id));
    } else {
      await db.insert(companies).values({
        name: c.name,
        slug: c.slug,
        ats: c.ats,
        boardToken: c.board_token,
        careersUrl: c.careers_url ?? null,
        websiteUrl: c.website_url ?? null,
        active: c.active ?? true,
      });
    }
    upserted += 1;
  }
  return { upserted, total: bySlug.size };
}
