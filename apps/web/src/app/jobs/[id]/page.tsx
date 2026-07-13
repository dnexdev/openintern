import { notFound, redirect } from "next/navigation";
import { jobPath } from "@/lib/job-slug";
import { loadJobById } from "@/lib/load-job";

export const dynamic = "force-dynamic";

export default async function JobIdRedirectPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const job = await loadJobById(id).catch(() => null);
  if (!job) notFound();
  redirect(jobPath(job.companySlug, job.title, job.id));
}
