import { createHash } from "node:crypto";

/**
 * Rule-based title canonicalization for role-family clustering.
 *
 * Strip *where* (office, campus site, region, season cohort).
 * Keep *what kind* and *who's eligible* (degree track, specialty/domain, clearance).
 */
export function normalizeTitle(raw: string): string {
  let s = raw.normalize("NFKC").trim();

  // Leading cohort year: "2027 - Software Engineering Intern …"
  s = s.replace(/^\s*20\d{2}\s*[-–—]\s*/i, "").trim();

  // Drop trailing parentheticals when they are season/program noise — not degree eligibility.
  let prev = "";
  while (s !== prev) {
    prev = s;
    s = s.replace(/\s*\(([^)]*)\)\s*$/g, (match, inner: string) =>
      shouldStripParenthetical(inner.trim()) ? "" : match,
    ).trim();
  }

  // Drop trailing " - Segment" / " – Segment" / " — Segment" when it looks like
  // region/program/clearance noise (not a core role qualifier).
  prev = "";
  while (s !== prev) {
    prev = s;
    const m = s.match(/^(.*?)(?:\s+[-–—]\s+)([^–—-]+)$/);
    if (!m) break;
    const head = m[1]!.trim();
    const tail = m[2]!.trim();
    if (isNoiseSuffix(tail) || isCampusSuffix(tail)) {
      s = head;
    } else {
      break;
    }
  }

  // Drop trailing ", Segment" when campus/university noise (e.g. ", IIT Madras").
  prev = "";
  while (s !== prev) {
    prev = s;
    const m = s.match(/^(.*?)(?:,\s*)([^,]+)$/);
    if (!m) break;
    const head = m[1]!.trim();
    const tail = m[2]!.trim();
    if (isNoiseSuffix(tail) || isCampusSuffix(tail)) {
      s = head;
    } else {
      break;
    }
  }

  // Generic campus-recruiting program prefix (Jump-style), not a location.
  s = s.replace(/^campus\s+/i, "").trim();

  // Strip inline campus tokens (IIT Madras, BITS Pilani, …)
  s = s.replace(/\b(?:iit|bits|nit|iiit)\s+[\p{L}\p{N}&'.-]+/giu, " ");
  s = s.replace(
    /\b(fall|winter|summer|spring|autumn)\s*(20\d{2})?\b/gi,
    " ",
  );
  s = s.replace(/\b20\d{2}\b/g, " ");

  // Light intern wording normalize
  s = s.replace(/\binternships?\b/gi, "intern");
  s = s.replace(/\bco-?ops?\b/gi, "coop");

  s = s
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\s+/&]/gu, " ")
    .replace(/\s+/g, " ")
    .trim();

  return s;
}

const NOISE_WORDS = new Set([
  "commercial",
  "usg",
  "intel",
  "government",
  "aus",
  "uk",
  "eu",
  "emea",
  "apac",
  "americas",
  "federal",
  "cleared",
  "secret",
  "ts",
  "sci",
  "tssci",
  "public",
  "sector",
  "enterprise",
  "startup",
  "early",
  "career",
  "remote",
  "hybrid",
  "onsite",
  "on-site",
  "united",
  "states",
  "usa",
  "canada",
  "kingdom",
  "australia",
  "germany",
  "france",
  "india",
  "singapore",
  "japan",
  "korea",
  "netherlands",
  "ireland",
  "sweden",
  "switzerland",
]);

/** Single- or two-word office cities in quant intern titles (DMC-style). */
const OFFICE_CITIES = new Set([
  "amsterdam",
  "austin",
  "boston",
  "chicago",
  "dublin",
  "hong kong",
  "london",
  "miami",
  "montreal",
  "new york",
  "paris",
  "san francisco",
  "seattle",
  "singapore",
  "sydney",
  "toronto",
  "zurich",
]);

/** Campus / university program suffixes (quant campus recruiting, India programs, etc.). */
function isCampusSuffix(tail: string): boolean {
  const t = tail.toLowerCase().trim();
  if (!t) return false;
  if (/^(iit|bits|nit|iiit)\b/.test(t)) return true;
  if (t === "campus") return true;
  if (/\buniversity\b/.test(t)) return true;
  if (/^campus\s+(recruiting|program|hire|hiring)\b/.test(t)) return true;
  return false;
}

/** Degree / eligibility label — preserved in normalized title. */
function isDegreeLabel(text: string): boolean {
  const t = text.trim().toLowerCase();
  if (!t) return false;
  if (
    /^(phd\/postdoc|phd|postdoc|ug\/ms|ug|ms|bs|mba|m\.?s\.?|b\.?s\.?|bs\/ms|bs\/ms\/phd|undergraduate|graduate|masters?)$/.test(
      t,
    )
  ) {
    return true;
  }
  return /^(bs|ms|phd|ug|mba|postdoc)(\s*\/\s*(bs|ms|phd|ug|mba|postdoc))+$/i.test(t);
}

/** Season + cohort year tail: "Summer 2027", "Fall 2026". */
function isSeasonCohortSuffix(tail: string): boolean {
  const t = tail.toLowerCase().trim();
  if (!t) return false;
  if (/^20\d{2}$/.test(t)) return true;
  return /^(fall|winter|summer|spring|autumn)\s+20\d{2}$/.test(t);
}

function shouldStripParenthetical(inner: string): boolean {
  if (!inner) return false;
  if (isDegreeLabel(inner)) return false;
  if (/^internships?$/i.test(inner)) return true;
  if (isSeasonCohortSuffix(inner)) return true;
  if (/^(fall|winter|summer|spring|autumn)(\s+20\d{2})?$/i.test(inner)) return true;
  if (isNoiseSuffix(inner)) return true;
  return false;
}

function isNoiseSuffix(tail: string): boolean {
  const t = tail.toLowerCase().trim();
  if (!t || t.length > 48) return false;

  // Explicit multi-word program / clearance / region labels
  if (
    /^(commercial|usg|intel|government|aus|uk|eu|emea|apac|americas|federal|cleared|secret|ts\/?sci|public\s+sector|enterprise|startup|early\s+career|aus\s+government|us\s+government)$/i.test(
      t,
    )
  ) {
    return true;
  }

  // All tokens are region/program noise (e.g. "AUS Government")
  const words = t.split(/[\s/]+/).filter(Boolean);
  if (
    words.length >= 1 &&
    words.length <= 4 &&
    words.every((w) => NOISE_WORDS.has(w))
  ) {
    return true;
  }

  // City / country-ish: "Chicago, IL", "Sydney, Australia"
  if (/^[A-Za-z .'-]+,\s*[A-Za-z .'-]+$/.test(tail)) return true;
  if (
    /^(remote|hybrid|onsite|on-site|united states|usa|canada|uk|united kingdom|australia|germany|france|india|singapore|japan|korea|netherlands|ireland|sweden|switzerland)$/i.test(
      t,
    )
  ) {
    return true;
  }

  // Short all-caps / code-like tokens (USG, AUS, NYC)
  if (/^[A-Z0-9]{2,6}$/.test(tail) && tail === tail.toUpperCase()) return true;

  if (isSeasonCohortSuffix(tail)) return true;

  if (OFFICE_CITIES.has(t)) return true;

  if (isCampusSuffix(tail)) return true;

  return false;
}

/** Stable short hash for role_family_id suffix. */
export function shortHash(input: string, len = 10): string {
  return createHash("sha256").update(input).digest("hex").slice(0, len);
}

export function jobFingerprint(
  companyId: string,
  source: string,
  externalJobId: string,
): string {
  return createHash("sha256")
    .update(`${companyId}|${source}|${externalJobId}`)
    .digest("hex");
}

export function roleFamilyId(companySlug: string, normalizedTitle: string): string {
  return `${companySlug}:${shortHash(normalizedTitle)}`;
}
