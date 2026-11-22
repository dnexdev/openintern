const REPO = "https://github.com/dnexdev/openintern/issues/new";

export function reportIssueUrl(input: {
  jobId: string;
  title: string;
  companyName: string;
  applyUrl: string;
  pageUrl: string;
}): string {
  // Prefills the "Report a listing" issue form (.github/ISSUE_TEMPLATE/listing.yml).
  const params = new URLSearchParams({
    template: "listing.yml",
    title: `Listing: ${input.companyName} — ${input.title}`.slice(0, 120),
    url: `${input.pageUrl} (job ${input.jobId})`,
    details: `Apply URL: ${input.applyUrl}\n\n`,
  });
  return `${REPO}?${params.toString()}`;
}
