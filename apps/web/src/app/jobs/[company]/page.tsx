import { notFound, redirect } from "next/navigation";
import { jobPath } from "@/lib/job-slug";
import { loadJobById } from "@/lib/load-job";

export const dynamic = "force-dynamic";

const UUID = /^[a-f0-9]{8}-[a-f0-9]{4}-[1-5][a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$/i;

/** Redirect legacy `/jobs/{uuid}` links to the canonical company/title URL. */
export default async function LegacyJobIdRedirectPage({
  params,
}: {
  params: Promise<{ company: string }>;
}) {
  const { company: legacyId } = await params;
  if (!UUID.test(legacyId)) notFound();

  const { job } = await loadJobById(legacyId);
  if (!job) notFound();
  redirect(jobPath(job.companySlug, job.title, job.id));
}
