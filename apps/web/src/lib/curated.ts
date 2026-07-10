import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import YAML from "yaml";
import { z } from "zod";

const tier1CuratedSchema = z.object({
  slugs: z.array(z.string().min(1)).min(1),
});

const __dirname = path.dirname(fileURLToPath(import.meta.url));

function tier1Path(): string {
  return path.resolve(__dirname, "../../../../data/curated/tier-1.yaml");
}

let cached: Set<string> | null = null;

export function getTier1Slugs(): Set<string> {
  if (cached) return cached;
  const raw = fs.readFileSync(tier1Path(), "utf8");
  const { slugs } = tier1CuratedSchema.parse(YAML.parse(raw));
  cached = new Set(slugs);
  return cached;
}

export function isTier1Employer(slug: string | null | undefined): boolean {
  if (!slug) return false;
  return getTier1Slugs().has(slug);
}
