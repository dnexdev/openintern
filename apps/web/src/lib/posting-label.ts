/** Campus site embedded in a job title (India quant campus recruiting). */
const CAMPUS_IN_TITLE =
  /(?:^|[,–—-]\s*)((?:IIT|BITS|NIT|IIIT)\s+[\p{L}\p{N}&'.-]+)/iu;

export function extractCampusFromTitle(title: string): string | null {
  const m = title.match(CAMPUS_IN_TITLE);
  return m?.[1]?.trim() ?? null;
}

export type FamilyPosting = {
  id: string;
  title: string;
  location: string;
  locations: string[];
  applyUrl: string;
};

export type ApplyRow = {
  key: string;
  jobId: string;
  label: string;
  applyUrl: string;
  appliedTitle: string;
};

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

export function formatPostingLocations(
  locations: string[],
  isRemote: boolean,
): string {
  const filtered = locations.map((l) => l.trim()).filter(Boolean);
  if (filtered.length > 0) return filtered.join(" · ");
  return isRemote ? "Remote" : "Location n/a";
}

/** True when the user must pick a row — never send Apply to an arbitrary posting. */
export function needsApplyPicker(postings: FamilyPosting[]): boolean {
  if (postings.length > 1) return true;
  const p = postings[0];
  return p ? p.locations.filter((l) => l.trim()).length > 1 : false;
}

/** Rows for the inline apply picker (one row per posting, or one row listing sub-locations). */
export function familyApplyRows(
  familyTitle: string,
  postings: FamilyPosting[],
): ApplyRow[] {
  if (postings.length === 0) return [];

  if (postings.length === 1) {
    const p = postings[0]!;
    const locs = p.locations.map((l) => l.trim()).filter(Boolean);
    const label =
      locs.length > 0 ? locs.join(" · ") : p.location || "Location n/a";
    return [
      {
        key: p.id,
        jobId: p.id,
        label,
        applyUrl: p.applyUrl,
        appliedTitle:
          locs.length > 1 ? `${familyTitle} (${label})` : familyTitle,
      },
    ];
  }

  return postings.map((p) => {
    const label = postingDisplayLabel(p, postings);
    return {
      key: p.id,
      jobId: p.id,
      label,
      applyUrl: p.applyUrl,
      appliedTitle: `${familyTitle} (${label})`,
    };
  });
}

/** Collapsed header copy when a family has multiple postings. */
export function familyPostingsLabel(
  postingCount: number,
  uniqueLocationCount: number,
): string {
  if (uniqueLocationCount < postingCount) {
    return `${postingCount} opening${postingCount === 1 ? "" : "s"}`;
  }
  return `${postingCount} location${postingCount === 1 ? "" : "s"}`;
}

export function familyMetaLabel(
  familyTitle: string,
  postings: FamilyPosting[],
): string {
  if (needsApplyPicker(postings)) {
    if (postings.length > 1) {
      const uniqueLocationCount = new Set(postings.map((p) => p.location)).size;
      return familyPostingsLabel(postings.length, uniqueLocationCount);
    }
    const locs = postings[0]!.locations.filter((l) => l.trim());
    return `${locs.length} location${locs.length === 1 ? "" : "s"}`;
  }
  return familyApplyRows(familyTitle, postings)[0]?.label ?? "Location n/a";
}
