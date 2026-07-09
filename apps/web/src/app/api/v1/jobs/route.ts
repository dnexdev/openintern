import { NextResponse } from "next/server";
import { and, desc, eq, gte, ilike, sql } from "drizzle-orm";
import { companies, jobs } from "@openintern/db";
import { getDb } from "@/lib/db";
import { clientIp, rateLimit } from "@/lib/rate-limit";

export const dynamic = "force-dynamic";

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
  const location = url.searchParams.get("location")?.trim() || undefined;
  const company = url.searchParams.get("company")?.trim() || undefined;
  const remote = url.searchParams.get("remote");
  const postedAfter = url.searchParams.get("posted_after");
  const page = Math.max(1, Number(url.searchParams.get("page") ?? 1));
  const limit = Math.min(100, Math.max(1, Number(url.searchParams.get("limit") ?? 25)));
  const offset = (page - 1) * limit;

  const conditions = [eq(jobs.isActive, true)];
  if (q) conditions.push(ilike(jobs.title, `%${q}%`));
  if (company) conditions.push(eq(companies.slug, company));
  if (remote === "true" || remote === "1") conditions.push(eq(jobs.isRemote, true));
  if (postedAfter) {
    const d = new Date(postedAfter);
    if (!Number.isNaN(d.getTime())) conditions.push(gte(jobs.postedAt, d));
  }
  if (location) {
    conditions.push(
      sql`exists (
        select 1 from jsonb_array_elements_text(${jobs.locations}) loc
        where loc ilike ${"%" + location + "%"}
      )`,
    );
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
