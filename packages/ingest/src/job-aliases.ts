/**
 * After companies merge (data/company-aliases.tsv) and ingest has run, match
 * each old company's jobs to the same postings under the surviving company and
 * append them to data/job-aliases.tsv. Old job links then redirect and applied
 * marks move over. Re-running is safe: existing pairs are kept.
 *
 *   DATABASE_URL=... pnpm job-aliases
 */
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { and, eq, inArray } from "drizzle-orm";
import { companies, createDbClient, jobs, type Db } from "@openintern/db";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, "../../..");

export async function findJobAliases(db: Db, companyAliases: [string, string][]): Promise<[string, string][]> {
  const pairs: [string, string][] = [];
  for (const [oldSlug, newSlug] of companyAliases) {
    const [oldCo] = await db.select().from(companies).where(eq(companies.slug, oldSlug));
    const [newCo] = await db.select().from(companies).where(eq(companies.slug, newSlug));
    if (!oldCo || !newCo) continue;
    const oldJobs = await db
      .select({ id: jobs.id, ext: jobs.externalJobId })
      .from(jobs)
      .where(eq(jobs.companyId, oldCo.id));
    if (oldJobs.length === 0) continue;
    const newJobs = await db
      .select({ id: jobs.id, ext: jobs.externalJobId })
      .from(jobs)
      .where(and(eq(jobs.companyId, newCo.id), inArray(jobs.externalJobId, oldJobs.map((j) => j.ext))));
    const byExt = new Map(newJobs.map((j) => [j.ext, j.id]));
    for (const j of oldJobs) {
      const target = byExt.get(j.ext);
      if (target && target !== j.id) pairs.push([j.id, target]);
    }
  }
  return pairs;
}

async function main() {
  const read = async (f: string) => fs.readFile(path.join(root, f), "utf8");
  const companyAliases = (await read("data/company-aliases.tsv"))
    .split("\n")
    .filter((l) => l.trim() && !l.startsWith("#") && !l.startsWith("old_slug\t"))
    .map((l) => l.split("\t").slice(0, 2) as [string, string]);
  const file = path.join(root, "data/job-aliases.tsv");
  const existing = await fs.readFile(file, "utf8");
  const known = new Set(existing.split("\n").map((l) => l.split("\t")[0]));
  const { db, client } = createDbClient();
  try {
    const fresh = (await findJobAliases(db, companyAliases)).filter(([o]) => !known.has(o));
    if (fresh.length) {
      await fs.appendFile(file, fresh.map(([o, n]) => `${o}\t${n}\n`).join(""));
    }
    console.log(`job-aliases: ${fresh.length} new pair(s); run pnpm gen-aliases and commit`);
  } finally {
    await client.end();
  }
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) await main();
