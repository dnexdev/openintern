import { jobSlugSegment as provenSegment, parseIdPrefix } from "@openintern/proven";

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
  return provenSegment(slugifyTitle(title) || "job", id);
}

export function jobPath(companySlug: string, title: string, id: string): string {
  return `/jobs/${companySlug}/${jobSlugSegment(title, id)}`;
}

/** Parse trailing 8-char id prefix from a job slug segment. */
export function parseIdPrefixFromJobSlug(jobSlug: string): string | null {
  return parseIdPrefix(jobSlug);
}
