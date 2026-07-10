import { z } from "zod";

/**
 * Tier 1 employer slugs (FAANG-level or adjacent).
 * Keep in sync with data/curated/tier-1.yaml — validated by `pnpm validate-curated`.
 * Inlined (not fs.read) so the /jobs route works on Vercel serverless.
 */
const TIER1_SLUGS = [
  // Frontier AI / labs
  "openai",
  "anthropic",
  "cohere",
  "xai",
  "groq",
  "mistral",
  "deepmind",
  // Top quant / HFT
  "jane-street",
  "citadel",
  "two-sigma",
  "hrt",
  "jump-trading",
  "optiver",
  "imc",
  // Category-defining tech
  "stripe",
  "databricks",
  "palantir",
  "scale-ai",
  "anduril",
  "nvidia",
  "cloudflare",
  "snowflake",
  // Mobility / hard tech
  "spacex",
  "tesla",
  "waymo",
  // Strong tech
  "netflix",
  "airbnb",
  "figma",
  "uber",
] as const;

const tier1CuratedSchema = z.object({
  slugs: z.array(z.string().min(1)).min(1),
});

let cached: Set<string> | null = null;

export function getTier1Slugs(): Set<string> {
  if (cached) return cached;
  const { slugs } = tier1CuratedSchema.parse({ slugs: [...TIER1_SLUGS] });
  cached = new Set(slugs);
  return cached;
}

export function isTier1Employer(slug: string | null | undefined): boolean {
  if (!slug) return false;
  return getTier1Slugs().has(slug);
}
