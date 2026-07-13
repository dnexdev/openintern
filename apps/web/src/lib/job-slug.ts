/** Human-readable job URL segment: slugified title + first 8 chars of UUID. */
export function slugifyTitle(title: string): string {
  return title
    .normalize("NFKC")
    .toLowerCase()
    .replace(/[^\p{L}\p{N}]+/gu, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 80);
}

export function jobSlugSegment(title: string, id: string): string {
  const base = slugifyTitle(title) || "job";
  const suffix = id.replace(/-/g, "").slice(0, 8);
  return `${base}-${suffix}`;
}

export function jobPath(companySlug: string, title: string, id: string): string {
  return `/jobs/${companySlug}/${jobSlugSegment(title, id)}`;
}

/** Parse trailing 8-char id prefix from a job slug segment. */
export function parseIdPrefixFromJobSlug(jobSlug: string): string | null {
  const match = jobSlug.match(/-([a-f0-9]{8})$/i);
  return match?.[1]?.toLowerCase() ?? null;
}
