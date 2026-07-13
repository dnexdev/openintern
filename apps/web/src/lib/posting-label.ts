/** Campus site embedded in a job title (India quant campus recruiting). */
const CAMPUS_IN_TITLE =
  /(?:^|[,–—-]\s*)((?:IIT|BITS|NIT|IIIT)\s+[\p{L}\p{N}&'.-]+)/iu;

export function extractCampusFromTitle(title: string): string | null {
  const m = title.match(CAMPUS_IN_TITLE);
  return m?.[1]?.trim() ?? null;
}

type PostingLike = { title: string; location: string };

/** Label for one row in an expanded family — avoids duplicate location strings. */
export function postingDisplayLabel(
  posting: PostingLike,
  siblings: PostingLike[],
): string {
  const campus = extractCampusFromTitle(posting.title);
  const sameLocation = siblings.filter((p) => p.location === posting.location);

  if (campus && sameLocation.length > 1) return campus;
  if (campus) return `${campus} · ${posting.location}`;
  if (sameLocation.length > 1) return posting.title;

  return posting.location;
}

/** Collapsed header / button copy when a family has multiple postings. */
export function familyPostingsLabel(postingCount: number, uniqueLocationCount: number): string {
  if (uniqueLocationCount < postingCount) {
    return `${postingCount} opening${postingCount === 1 ? "" : "s"}`;
  }
  return `${postingCount} location${postingCount === 1 ? "" : "s"}`;
}

export function familyPostingsToggle(uniqueLocationCount: number, postingCount: number): string {
  return uniqueLocationCount < postingCount ? "Show openings" : "Show locations";
}
