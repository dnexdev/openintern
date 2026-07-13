import { NextResponse } from "next/server";
import { loadJobFamilies, type FamilySort } from "@/lib/job-families";
import { clientIp, rateLimit } from "@/lib/rate-limit";

export const dynamic = "force-dynamic";

const ROLE_SET = new Set([
  "software",
  "backend",
  "frontend",
  "fullstack",
  "data",
  "ml",
  "mobile",
  "security",
  "devops",
  "hardware",
  "quant",
  "product",
  "research",
]);
const REGION_SET = new Set(["remote", "us", "canada", "europe", "other"]);
const SEASON_SET = new Set(["summer", "fall", "winter"]);

function valuesFor(params: URLSearchParams, ...keys: string[]) {
  return keys.flatMap((key) => params.getAll(key)).flatMap((value) => value.split(","));
}

function positiveInt(value: string | null, fallback: number) {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
}

export async function GET(request: Request) {
  const ip = clientIp(request);
  const rl = rateLimit(`jobs:${ip}`);
  if (!rl.ok) {
    return NextResponse.json(
      { error: "Rate limit exceeded. Use daily dumps or self-host for bulk access." },
      {
        status: 429,
        headers: {
          "X-RateLimit-Remaining": "0",
          "X-RateLimit-Reset": String(rl.resetAt),
          "Retry-After": String(Math.ceil((rl.resetAt - Date.now()) / 1000)),
        },
      },
    );
  }

  const url = new URL(request.url);
  const q = url.searchParams.get("q")?.trim() || "";
  const company = url.searchParams.get("company")?.trim() || "";
  const postedAfterRaw = url.searchParams.get("posted_after");
  let postedAfter: Date | null = null;
  if (postedAfterRaw) {
    const d = new Date(postedAfterRaw);
    if (!Number.isNaN(d.getTime())) postedAfter = d;
  }
  const roles = url.searchParams
    .getAll("role")
    .flatMap((s) => s.split(","))
    .map((s) => s.trim().toLowerCase())
    .filter((s) => ROLE_SET.has(s));
  const regions = url.searchParams
    .getAll("region")
    .flatMap((s) => s.split(","))
    .map((s) => s.trim().toLowerCase())
    .filter((s) => REGION_SET.has(s));
  const seasons = valuesFor(url.searchParams, "season", "term")
    .map((s) => s.trim().toLowerCase())
    .map((s) => (s === "spring" ? "summer" : s === "autumn" ? "fall" : s))
    .filter((s) => SEASON_SET.has(s));
  const durations = valuesFor(url.searchParams, "duration_months", "duration")
    .map(Number)
    .filter((n) => Number.isInteger(n) && n >= 1 && n <= 24);
  const page = positiveInt(url.searchParams.get("page"), 1);
  const limit = Math.min(100, positiveInt(url.searchParams.get("limit"), 27));
  const offset = (page - 1) * limit;
  const sortRaw = url.searchParams.get("sort");
  const sort: FamilySort =
    sortRaw === "posted"
      ? "posted"
      : sortRaw === "prestige"
        ? "prestige"
        : "first_seen";

  const { families, total } = await loadJobFamilies({
    query: q,
    company,
    roles,
    regions,
    terms: seasons,
    durations,
    postedAfter,
    sort,
    limit,
    offset,
  });

  const totalPages = Math.max(1, Math.ceil(total / limit));

  return NextResponse.json(
    {
      jobs: families.map((f) => ({
        role_family_id: f.roleFamilyId,
        company: {
          name: f.company.name,
          slug: f.company.slug,
          ats: f.company.ats,
        },
        title: f.title,
        postings: f.postings.map((p) => ({
          id: p.id,
          title: p.title,
          location: p.location,
          posted_at: p.postedAt,
          apply_url: p.applyUrl,
        })),
      })),
      page,
      limit,
      total,
      total_pages: totalPages,
      has_more: page < totalPages,
    },
    {
      headers: {
        "X-RateLimit-Remaining": String(rl.remaining),
        "Cache-Control": "public, s-maxage=60, stale-while-revalidate=300",
      },
    },
  );
}
