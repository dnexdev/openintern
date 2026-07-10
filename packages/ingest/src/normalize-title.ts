import { createHash } from "node:crypto";

/**
 * Rule-based title canonicalization for role-family clustering.
 * Strips location/program/season noise so suffix variants share a family.
 */
export function normalizeTitle(raw: string): string {
  let s = raw.normalize("NFKC").trim();

  // Drop trailing parentheticals repeatedly: (Fall 2026), (AUS Government), …
  let prev = "";
  while (s !== prev) {
    prev = s;
    s = s.replace(/\s*\([^)]*\)\s*$/g, "").trim();
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
    if (isNoiseSuffix(tail)) {
      s = head;
    } else {
      break;
    }
  }

  // Strip inline season + year tokens
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
