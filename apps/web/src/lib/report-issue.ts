const REPO = "https://github.com/dnexdev/openintern/issues/new";

export function reportIssueUrl(input: {
  jobId: string;
  title: string;
  companyName: string;
  applyUrl: string;
  pageUrl: string;
}): string {
  const issueTitle = `Listing: ${input.companyName} — ${input.title}`.slice(0, 120);
  const body = [
    "## Listing",
    "",
    `- Job ID: \`${input.jobId}\``,
    `- OpenIntern: ${input.pageUrl}`,
    `- Apply URL: ${input.applyUrl}`,
    "",
    "## Issue",
    "",
    "<!-- stale listing, wrong location, not an internship, etc. -->",
    "",
  ].join("\n");

  const params = new URLSearchParams({
    title: issueTitle,
    body,
  });
  return `${REPO}?${params.toString()}`;
}
