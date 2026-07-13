import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { JobDetailView } from "@/components/JobDetailView";
import { jobPath, parseIdPrefixFromJobSlug } from "@/lib/job-slug";
import { loadJobByCompanyAndIdPrefix } from "@/lib/load-job";

export const dynamic = "force-dynamic";

function jobDescription(job: {
  title: string;
  companyName: string;
  excerpt: string | null;
  locations: string[];
}) {
  const loc = job.locations?.length ? job.locations.join(", ") : "Location n/a";
  return (
    job.excerpt?.slice(0, 160) ||
    `${job.title} at ${job.companyName} (${loc}) — tech internship on OpenIntern.`
  );
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ company: string; jobSlug: string }>;
}): Promise<Metadata> {
  const { company, jobSlug } = await params;
  const idPrefix = parseIdPrefixFromJobSlug(jobSlug);
  if (!idPrefix) return { title: "Job not found · OpenIntern" };

  try {
    const job = await loadJobByCompanyAndIdPrefix(company, idPrefix);
    if (!job) return { title: "Job not found · OpenIntern" };

    const description = jobDescription(job);
    const canonical = jobPath(job.companySlug, job.title, job.id);
    return {
      title: `${job.title} at ${job.companyName} · OpenIntern`,
      description,
      alternates: { canonical },
      openGraph: {
        title: `${job.title} at ${job.companyName}`,
        description,
        type: "article",
        url: canonical,
      },
    };
  } catch {
    return { title: "OpenIntern" };
  }
}

export default async function JobSlugDetailPage({
  params,
}: {
  params: Promise<{ company: string; jobSlug: string }>;
}) {
  const { company, jobSlug } = await params;
  const idPrefix = parseIdPrefixFromJobSlug(jobSlug);
  if (!idPrefix) notFound();

  const job = await loadJobByCompanyAndIdPrefix(company, idPrefix).catch(() => null);
  if (!job) notFound();

  return <JobDetailView job={job} />;
}
