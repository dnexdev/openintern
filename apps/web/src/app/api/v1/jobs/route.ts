import { NextResponse } from "next/server";
import { and, desc, eq, gte, ilike, or, sql, type SQL } from "drizzle-orm";
import { companies, jobs } from "@openintern/db";
import { getDb } from "@/lib/db";
import { freshnessSql } from "@/lib/freshness";
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
  const q = url.searchParams.get("q")?.trim() || undefined;
  const company = url.searchParams.get("company")?.trim() || undefined;
  const postedAfter = url.searchParams.get("posted_after");
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
  const seasons = url.searchParams
    .getAll("season")
    .flatMap((s) => s.split(","))
    .map((s) => s.trim().toLowerCase())
    .map((s) => (s === "spring" ? "summer" : s === "autumn" ? "fall" : s))
    .filter((s) => SEASON_SET.has(s));
  const durations = url.searchParams
    .getAll("duration_months")
    .flatMap((s) => s.split(","))
    .map(Number)
    .filter((n) => Number.isInteger(n) && n >= 1 && n <= 24);
  const page = Math.max(1, Number(url.searchParams.get("page") ?? 1));
  const limit = Math.min(100, Math.max(1, Number(url.searchParams.get("limit") ?? 25)));
  const offset = (page - 1) * limit;

  const conditions: (SQL | undefined)[] = [eq(jobs.isActive, true), freshnessSql()];

  if (q) conditions.push(ilike(jobs.title, `%${q}%`));
  if (company) conditions.push(eq(companies.slug, company));
  if (roles.length > 0) {
    conditions.push(
      or(...roles.map((r) => sql`${jobs.roles} @> ${JSON.stringify([r])}::jsonb`)),
    );
  }
  if (regions.length > 0) {
    conditions.push(
      or(...regions.map((r) => sql`${jobs.regions} @> ${JSON.stringify([r])}::jsonb`)),
    );
  }
  if (seasons.length > 0) {
    conditions.push(
      or(...seasons.map((s) => sql`${jobs.terms} @> ${JSON.stringify([s])}::jsonb`)),
    );
  }
  if (durations.length > 0) {
    conditions.push(
      or(
        ...durations.map(
          (d) => sql`${jobs.durationMonths} @> ${JSON.stringify([d])}::jsonb`,
        ),
      ),
    );
  }
  if (postedAfter) {
    const d = new Date(postedAfter);
    if (!Number.isNaN(d.getTime())) conditions.push(gte(jobs.postedAt, d));
  }

  const db = getDb();
  const where = and(...conditions);

  const [countRow] = await db
    .select({ count: sql<number>`count(*)::int` })
    .from(jobs)
    .innerJoin(companies, eq(jobs.companyId, companies.id))
    .where(where);

  const rows = await db
    .select({
      id: jobs.id,
      title: jobs.title,
      locations: jobs.locations,
      apply_url: jobs.applyUrl,
      excerpt: jobs.excerpt,
      terms: jobs.terms,
      term_years: jobs.termYears,
      duration_months: jobs.durationMonths,
      cohort_year: jobs.cohortYear,
      roles: jobs.roles,
      regions: jobs.regions,
      is_remote: jobs.isRemote,
      source: jobs.source,
      posted_at: jobs.postedAt,
      first_seen_at: jobs.firstSeenAt,
      last_seen_at: jobs.lastSeenAt,
      company: {
        name: companies.name,
        slug: companies.slug,
        ats: companies.ats,
      },
    })
    .from(jobs)
    .innerJoin(companies, eq(jobs.companyId, companies.id))
    .where(where)
    .orderBy(desc(jobs.firstSeenAt))
    .limit(limit)
    .offset(offset);

  return NextResponse.json(
    {
      data: rows.map((r) => ({
        ...r,
        posted_at: r.posted_at?.toISOString() ?? null,
        first_seen_at: r.first_seen_at.toISOString(),
        last_seen_at: r.last_seen_at.toISOString(),
      })),
      page,
      limit,
      total: countRow?.count ?? 0,
    },
    {
      headers: {
        "X-RateLimit-Remaining": String(rl.remaining),
        "Cache-Control": "public, s-maxage=60, stale-while-revalidate=300",
      },
    },
  );
}
