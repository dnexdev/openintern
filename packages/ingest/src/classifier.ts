/**
 * Heuristic internship classifier for tech roles.
 * Internship signal must come from the TITLE (intern/co-op/apprentice);
 * new-grad/full-time roles are out of scope for this board.
 */

/** Internship signal — title only. */
const INTERNSHIP_TITLE =
  /\b(intern|internship|co-?op|coop|apprentice(ship)?)\b/i;

/** Seniority noise — title only. */
const EXCLUDE =
  /\b(senior|staff|principal|director|manager|lead|head of|vp\b|vice president|chief|architect)\b/i;

const TECH_HINT =
  /\b(software|engineer|engineering|developer|swe|sde|frontend|backend|full[\s-]?stack|data|ml|machine learning|ai|artificial intelligence|research|security|infra|infrastructure|devops|sre|platform|mobile|ios|android|firmware|hardware|embedded|quant|quantitative|product\s*manager|pm\b|design\s*engineer|site reliability|robotics|computer\s*(science|vision)|cyber|cloud|analytics|scientist)\b/i;

/** Clearly non-tech domains — if the title says this and has no tech hint, drop it. */
const NEG_DOMAIN =
  /\b(marketing|sales|account\s*(executive|manager)|hr\b|human\s*resources|people\s*(ops|operations|team)?|talent|recruit(er|ing|ment)|finance|financial|accounting|accountant|audit|tax\b|legal|paralegal|counsel|customer\s*(success|support|service|experience)|business\s*development|bd\b|partnerships?|community|content|copywrit|social\s*media|brand|communications?|pr\b|public\s*relations|events?\s*(coordinator|manager|intern)|admin(istrative)?|office\s*(manager|coordinator)|executive\s*assistant|retail|store|warehouse|logistics|supply\s*chain|procurement|facilities|culinary|barista|driver|electrician|plumber|hvac|mechanic|weld(er|ing)?|machinist|assembler|janitor|custodian|landscap|nurse|clinical|pharmac)\b/i;

export function isTechInternship(title: string, description = ""): boolean {
  // 1. Must be an internship by title — descriptions mention "interns" too loosely.
  if (!INTERNSHIP_TITLE.test(title)) return false;

  // 2. Drop senior/staff-titled roles unless the title is explicitly internship-shaped
  //    (which it is at this point) — keep the guard for weird "Intern Manager" listings.
  if (EXCLUDE.test(title)) return false;

  const titleTech = TECH_HINT.test(title);
  const titleNonTech = NEG_DOMAIN.test(title);

  // 3. Title states a domain — trust it.
  if (titleTech) return true;
  if (titleNonTech) return false;

  // 4. Generic title ("Intern", "Co-op Student") — fall back to the description.
  return TECH_HINT.test(description);
}

export function looksRemote(locations: string[], title = ""): boolean {
  const blob = `${locations.join(" ")} ${title}`.toLowerCase();
  return /\bremote\b|\bwfh\b|work from home|anywhere/.test(blob);
}

const TERM_ORDER = ["winter", "spring", "summer", "fall"] as const;

export type InternshipTerm = (typeof TERM_ORDER)[number];

/**
 * Extract internship seasons mentioned in the text (normalizes autumn -> fall).
 * Returned in canonical order: winter, spring, summer, fall.
 */
export function extractTerms(text: string): InternshipTerm[] {
  const found = new Set<InternshipTerm>();
  for (const match of text.matchAll(/\b(winter|spring|summer|fall|autumn)\b/gi)) {
    const raw = match[1].toLowerCase();
    found.add(raw === "autumn" ? "fall" : (raw as InternshipTerm));
  }
  return TERM_ORDER.filter((t) => found.has(t));
}

/**
 * Extract internship duration in months. Prefers explicit "N month(s)";
 * falls back to converting "N week(s)". Returns null when not stated.
 */
export function extractDurationMonths(text: string): number | null {
  const monthMatch = text.match(/\b(\d{1,2})\s*[-–]?\s*month\b/i);
  if (monthMatch) {
    const n = Number(monthMatch[1]);
    if (n >= 1 && n <= 24) return n;
  }
  const weekMatch = text.match(/\b(\d{1,2})\s*[-–]?\s*week\b/i);
  if (weekMatch) {
    const weeks = Number(weekMatch[1]);
    if (weeks >= 4 && weeks <= 96) return Math.round(weeks / 4.345);
  }
  return null;
}

function decodeEntities(text: string): string {
  return text
    .replace(/&nbsp;/gi, " ")
    .replace(/&quot;/gi, '"')
    .replace(/&#0?39;/g, "'")
    .replace(/&apos;/gi, "'")
    .replace(/&lt;/gi, "<")
    .replace(/&gt;/gi, ">")
    .replace(/&amp;/gi, "&");
}

function stripTags(text: string): string {
  return text
    .replace(/<script[\s\S]*?<\/script>/gi, " ")
    .replace(/<style[\s\S]*?<\/style>/gi, " ")
    .replace(/<[^>]+>/g, " ");
}

export function excerptFromHtml(html: string | undefined | null, max = 400): string | null {
  if (!html) return null;
  // Some sources double-encode: strip, decode, and strip again so
  // `<div class=&quot;...&quot;>` payloads don't leak markup into excerpts.
  let text = stripTags(html);
  text = decodeEntities(text);
  text = stripTags(text);
  text = decodeEntities(text);
  text = text.replace(/\s+/g, " ").trim();
  if (!text) return null;
  return text.length > max ? `${text.slice(0, max - 1)}…` : text;
}
