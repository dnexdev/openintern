import { NextResponse } from "next/server";
import { asc, eq, sql } from "drizzle-orm";
import { companies, jobs } from "@openintern/db";
import { getDb } from "@/lib/db";
import { clientIp, rateLimit } from "@/lib/rate-limit";

export const dynamic = "force-dynamic";

export async function GET(request: Request) {
  const ip = clientIp(request);
  const rl = rateLimit(`companies:${ip}`);
  if (!rl.ok) {
    return NextResponse.json({ error: "Rate limit exceeded" }, { status: 429 });
  }

  const url = new URL(request.url);
  const activeOnly = url.searchParams.get("active") !== "false";

  const db = getDb();
  const rows = await db
    .select({
      id: companies.id,
      name: companies.name,
      slug: companies.slug,
      ats: companies.ats,
      careers_url: companies.careersUrl,
      website_url: companies.websiteUrl,
      active: companies.active,
      active_jobs: sql<number>`(
        select count(*)::int from ${jobs}
        where ${jobs.companyId} = ${companies.id} and ${jobs.isActive} = true
      )`,
    })
    .from(companies)
    .where(activeOnly ? eq(companies.active, true) : undefined)
    .orderBy(asc(companies.name));

  return NextResponse.json({ data: rows, total: rows.length });
}
