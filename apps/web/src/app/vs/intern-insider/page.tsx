import type { Metadata } from "next";
import Link from "next/link";
import { InternInsiderComparison } from "@/components/InternInsiderComparison";

export const metadata: Metadata = {
  title: "OpenIntern vs Intern Insider",
  description:
    "Compare OpenIntern (free, open-source, public API) with Intern Insider (paid subscription, gated board).",
  alternates: { canonical: "/vs/intern-insider" },
};

export default function VsInternInsiderPage() {
  return (
    <article className="prose-page">
      <p className="back-link">
        <Link href="/">← Home</Link>
      </p>
      <h1>OpenIntern vs Intern Insider</h1>
      <p>
        OpenIntern is a free, open-source tech internship board with a public API and
        daily dumps. No account required to browse. Apply links always go to the
        employer.
      </p>
      <InternInsiderComparison />
      <p>
        <Link href="/jobs" className="btn btn-primary">
          Browse internships
        </Link>
      </p>
    </article>
  );
}
