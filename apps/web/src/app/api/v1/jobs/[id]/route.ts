import { NextResponse } from "next/server";
import { and, eq } from "drizzle-orm";
import { companies, jobs } from "@openintern/db";
import { getDb } from "@/lib/db";
import { clientIp, rateLimit } from "@/lib/rate-limit";

export const dynamic = "force-dynamic";

export async function GET(
  request: Request,
  context: { params: Promise<{ id: string }> },
) {
  const ip = clientIp(request);
  const rl = rateLimit(`job:${ip}`);
  if (!rl.ok) {
    return NextResponse.json({ error: "Rate limit exceeded" }, { status: 429 });
  }

  const { id } = await context.params;
  const db = getDb();
  const [row] = await db
    .select({
      id: jobs.id,
      title: jobs.title,
      locations: jobs.locations,
      apply_url: jobs.applyUrl,
      excerpt: jobs.excerpt,
      is_remote: jobs.isRemote,
      is_active: jobs.isActive,
      source: jobs.source,
      posted_at: jobs.postedAt,
      first_seen_at: jobs.firstSeenAt,
      last_seen_at: jobs.lastSeenAt,
      company: {
        name: companies.name,
        slug: companies.slug,
        ats: companies.ats,
        careers_url: companies.careersUrl,
      },
    })
    .from(jobs)
    .innerJoin(companies, eq(jobs.companyId, companies.id))
    .where(and(eq(jobs.id, id)))
    .limit(1);

  if (!row) {
    return NextResponse.json({ error: "Not found" }, { status: 404 });
  }

  return NextResponse.json({
    data: {
      ...row,
      posted_at: row.posted_at?.toISOString() ?? null,
      first_seen_at: row.first_seen_at.toISOString(),
      last_seen_at: row.last_seen_at.toISOString(),
    },
  });
}
