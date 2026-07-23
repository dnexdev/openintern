/**
 * Heuristic internship classifier for tech roles.
 * Internship signal must come from the TITLE (intern/co-op/apprentice);
 * new-grad/full-time roles are out of scope for this board.
 */

/**
 * Internship signal — title only.
 * Also matches campus / project / "Year at X" internship phrasings common on public lists.
 */
const INTERNSHIP_TITLE =
  /\b(intern|internship|co-?op|coop|apprentice(ship)?|project\s+intern|campus\s+\w+\s+intern|year\s+at\s+\w+)\b/i;

/**
 * Seniority noise — title only.
 * "manager" alone is excluded, but Product/Program/Project Manager Intern is allowed.
 */
const EXCLUDE_SENIORITY =
  /\b(senior|staff|principal|director|lead|head of|vp\b|vice president|chief|architect)\b/i;
const EXCLUDE_MANAGER = /\bmanager\b/i;
const ALLOWED_MANAGER =
  /\b(product|program|project|technical\s*program)\s*managers?\b/i;

/** Non-tech / non-CS domains — drop even when a description mentions engineers. */
const NEG_DOMAIN =
  /\b(marketing|sales|\bsdr\b|account\s*(executive|manager)|hr\b|human\s*resources|people\s*(ops|operations|team)?|talent|recruit(er|ing|ment)|finance|financial|accounting|accountant|audit|tax\b|legal|paralegal|counsel|customer\s*(success|support|service|experience)|technical\s*support|business\s*development|bd\b|partnerships?|community|content|copywrit|social\s*media|brand|communications?|pr\b|public\s*relations|government\s*affairs|policy|events?\s*(coordinator|manager|intern)|admin(istrative)?|office\s*(manager|coordinator)|executive\s*assistant|retail|store|warehouse|logistics|supply\s*chain|procurement|facilities|culinary|barista|driver|electrician|plumber|hvac|mechanic|weld(er|ing)?|machinist|assembler|janitor|custodian|landscap|nurse|clinical|pharmac|industrial\s*engineer(ing)?|process\s*engineer(ing)?|manufacturing\s*engineer(ing)?|mechanical\s*engineer(ing)?|civil\s*engineer(ing)?|chemical\s*engineer(ing)?|quality\s*engineer(ing)?|tires?\b|lab\s*(technician|tech|assistant)|production\s*engineer(ing)?|trade\s*compliance|network\s*strategy|business\s*operations|(?:platform|business|content|marketing|growth|commercial|sales|brand|product)\s+strategy|product\s*operations|program\s*management|project\s*management|user\s*research|ux\s*(research|design))\b/i;

/**
 * Ambiguous titles may use the description only when the title itself says the
 * internship is technical. A plain "Project Intern" must never pass because a
 * long description happens to mention software or an engineering team.
 */
const GENERIC_TECH_TITLE =
  /\b(engineer(?:ing)?|technical|technology|computer\s+science|computing|systems)\s+(?:intern(?:ship)?|co-?op|coop|apprentice)\b|\b(?:intern(?:ship)?|co-?op|coop|apprentice)[,\s-]+(?:engineer(?:ing)?|technical|technology|computer\s+science|computing|systems)\b/i;

export function isTechInternship(title: string, description = ""): boolean {
  if (!INTERNSHIP_TITLE.test(title)) return false;
  if (EXCLUDE_SENIORITY.test(title)) return false;
  if (EXCLUDE_MANAGER.test(title) && !ALLOWED_MANAGER.test(title)) return false;
  if (NEG_DOMAIN.test(title)) return false;

  // Every admitted posting must map to at least one filterable tech role.
  if (extractRoles(title).length > 0) return true;

  // Description fallback is intentionally limited to explicitly technical but
  // otherwise generic titles such as "Engineering Internship".
  return GENERIC_TECH_TITLE.test(title) && extractRoles("", description).length > 0;
}

export function looksRemote(locations: string[], title = ""): boolean {
  const blob = `${locations.join(" ")} ${title}`.toLowerCase();
  return /\bremote\b|\bwfh\b|work from home|anywhere/.test(blob);
}

/** Canonical terms after spring→summer and autumn→fall. */
export const TERM_ORDER = ["summer", "fall", "winter"] as const;
export type InternshipTerm = (typeof TERM_ORDER)[number];

export type TermYear = { term: InternshipTerm; year: number };

export const ROLE_OPTIONS = [
  "software",
  "backend",
  "frontend",
  "fullstack",
  "data",
  "ml",
  "mobile",
  "security",
  "devops",
  "hardware",
  "quant",
  "product",
  "research",
] as const;
export type JobRole = (typeof ROLE_OPTIONS)[number];

export const REGION_OPTIONS = ["remote", "us", "canada", "europe", "other"] as const;
export type JobRegion = (typeof REGION_OPTIONS)[number];

export const DURATION_PRESETS = [3, 4, 6, 8, 12] as const;

function normalizeTermWord(raw: string): InternshipTerm {
  const w = raw.toLowerCase();
  if (w === "autumn" || w === "fall") return "fall";
  if (w === "spring" || w === "summer") return "summer";
  return "winter";
}

/**
 * Extract internship seasons. Spring → summer; autumn → fall.
 * Order: summer, fall, winter.
 */
export function extractTerms(text: string): InternshipTerm[] {
  const found = new Set<InternshipTerm>();
  for (const match of text.matchAll(/\b(winter|spring|summer|fall|autumn)\b/gi)) {
    found.add(normalizeTermWord(match[1]));
  }
  return TERM_ORDER.filter((t) => found.has(t));
}

/**
 * Season+year pairs from text (e.g. "Winter 2026", "Fall 2027").
 * Also pairs bare seasons with a cohort year when present.
 */
export function extractTermYears(text: string, cohortYear: number | null): TermYear[] {
  const pairs = new Map<string, TermYear>();
  const add = (term: InternshipTerm, year: number) => {
    const y = clampCohortYear(year);
    if (y == null) return;
    pairs.set(`${term}:${y}`, { term, year: y });
  };

  for (const match of text.matchAll(
    /\b(winter|spring|summer|fall|autumn)\s+(20\d{2})\b/gi,
  )) {
    add(normalizeTermWord(match[1]), Number(match[2]));
  }

  const terms = extractTerms(text);
  if (cohortYear != null && terms.length > 0) {
    for (const t of terms) {
      // Only backfill if we didn't already get an explicit season+year for this term
      const hasExplicit = [...pairs.values()].some((p) => p.term === t);
      if (!hasExplicit) add(t, cohortYear);
    }
  }

  return TERM_ORDER.flatMap((t) =>
    [...pairs.values()]
      .filter((p) => p.term === t)
      .sort((a, b) => a.year - b.year),
  );
}

/**
 * Season end date (last day of the term window) for freshness checks.
 * summer → Aug 31, fall → Dec 31, winter → Apr 30 of that year.
 */
export function termYearEndDate(ty: TermYear): Date {
  if (ty.term === "summer") return new Date(Date.UTC(ty.year, 7, 31, 23, 59, 59));
  if (ty.term === "fall") return new Date(Date.UTC(ty.year, 11, 31, 23, 59, 59));
  // winter = Jan–Apr of that year
  return new Date(Date.UTC(ty.year, 3, 30, 23, 59, 59));
}

/** True if every known term_year is entirely in the past. Empty → not stale. */
export function isStaleByTermYears(
  termYears: TermYear[],
  now: Date = new Date(),
): boolean {
  if (termYears.length === 0) return false;
  return termYears.every((ty) => termYearEndDate(ty).getTime() < now.getTime());
}

/** Max age for jobs with no term_years (days). */
export const MAX_AGE_DAYS_NO_TERM = 120;

/** Deactivate if not seen in this many days (covers failing company fetches). */
export const LAST_SEEN_STALE_DAYS = 14;

/**
 * Freshness beyond term_years: empty term_years + old posted/firstSeen → stale.
 */
export function isStaleByAge(opts: {
  termYears: TermYear[];
  postedAt: Date | null;
  firstSeenAt: Date;
  now?: Date;
  maxAgeDays?: number;
}): boolean {
  if (opts.termYears.length > 0) return false;
  const now = opts.now ?? new Date();
  const maxAgeDays = opts.maxAgeDays ?? MAX_AGE_DAYS_NO_TERM;
  const anchor = opts.postedAt ?? opts.firstSeenAt;
  const ageMs = now.getTime() - anchor.getTime();
  return ageMs > maxAgeDays * 24 * 60 * 60 * 1000;
}

export function isStaleByLastSeen(
  lastSeenAt: Date,
  now: Date = new Date(),
  maxDays: number = LAST_SEEN_STALE_DAYS,
): boolean {
  return now.getTime() - lastSeenAt.getTime() > maxDays * 24 * 60 * 60 * 1000;
}

/**
 * Duration values in months. Ranges like "4 to 6 months" expand to [4,5,6]
 * (clamped to 1–24). Prefer month phrasing; fall back to weeks.
 */
export function extractDurationMonthsList(text: string): number[] {
  const found = new Set<number>();
  const coveredSpans: [number, number][] = [];

  const addRange = (a: number, b: number) => {
    const lo = Math.min(a, b);
    const hi = Math.max(a, b);
    for (let n = lo; n <= hi; n++) {
      if (n >= 1 && n <= 24) found.add(n);
    }
  };

  for (const match of text.matchAll(
    /\b(\d{1,2})\s*(?:to|-|–|—)\s*(\d{1,2})\s*months?\b/gi,
  )) {
    if (match.index != null) {
      coveredSpans.push([match.index, match.index + match[0].length]);
    }
    addRange(Number(match[1]), Number(match[2]));
  }

  for (const match of text.matchAll(/\b(\d{1,2})\s*[-–—]?\s*months?\b/gi)) {
    if (match.index != null) {
      const inside = coveredSpans.some(
        ([s, e]) => match.index! >= s && match.index! < e,
      );
      if (inside) continue;
    }
    const n = Number(match[1]);
    if (n >= 1 && n <= 24) found.add(n);
  }

  if (found.size === 0) {
    for (const match of text.matchAll(
      /\b(\d{1,2})\s*(?:to|-|–|—)\s*(\d{1,2})\s*weeks?\b/gi,
    )) {
      const a = Math.round(Number(match[1]) / 4.345);
      const b = Math.round(Number(match[2]) / 4.345);
      addRange(a, b);
    }
    if (found.size === 0) {
      for (const match of text.matchAll(/\b(\d{1,2})\s*[-–—]?\s*weeks?\b/gi)) {
        const months = Math.round(Number(match[1]) / 4.345);
        if (months >= 1 && months <= 24) found.add(months);
      }
    }
  }

  return [...found].sort((a, b) => a - b);
}

/** @deprecated prefer extractDurationMonthsList */
export function extractDurationMonths(text: string): number | null {
  const list = extractDurationMonthsList(text);
  return list[0] ?? null;
}

function clampCohortYear(year: number): number | null {
  const now = new Date().getFullYear();
  if (year >= now - 1 && year <= now + 3) return year;
  return null;
}

/**
 * Extract program / cohort year (e.g. 2026 from "Summer 2026" or "Class of 2027").
 */
export function extractCohortYear(text: string): number | null {
  const seasonYear = text.match(
    /\b(?:winter|spring|summer|fall|autumn)\s+(20\d{2})\b/i,
  );
  if (seasonYear) {
    const y = clampCohortYear(Number(seasonYear[1]));
    if (y != null) return y;
  }

  const classOf = text.match(/\bclass of\s+(20\d{2})\b/i);
  if (classOf) {
    const y = clampCohortYear(Number(classOf[1]));
    if (y != null) return y;
  }

  const nearIntern = text.match(
    /\b(20\d{2})\b[^.]{0,40}\b(?:intern|internship|co-?op|coop|apprentice)/i,
  );
  if (nearIntern) {
    const y = clampCohortYear(Number(nearIntern[1]));
    if (y != null) return y;
  }
  const afterIntern = text.match(
    /\b(?:intern|internship|co-?op|coop|apprentice)[^.]{0,40}\b(20\d{2})\b/i,
  );
  if (afterIntern) {
    const y = clampCohortYear(Number(afterIntern[1]));
    if (y != null) return y;
  }

  return null;
}

const ROLE_PATTERNS: { role: JobRole; re: RegExp }[] = [
  { role: "fullstack", re: /\b(full[\s-]?stack)\b/i },
  { role: "frontend", re: /\b(frontend|front[\s-]?end|ui\s+engineer|web\s+(developer|engineer)|react|next\.?js)\b/i },
  { role: "backend", re: /\b(backend|back[\s-]?end|server[\s-]?side|api\s+engineer|distributed\s+systems)\b/i },
  { role: "ml", re: /\b(machine learning|\bml\b|deep learning|\bai\b|artificial intelligence|llm|nlp|natural language|computer vision|applied scientist)\b/i },
  { role: "data", re: /\b(data\s*(science|scientist|engineer|analytics|analyst)|analytics|business intelligence)\b/i },
  { role: "mobile", re: /\b(mobile|\bios\b|android|react native|flutter)\b/i },
  { role: "security", re: /\b(security|cyber|infosec|appsec|privacy\s+engineer)\b/i },
  { role: "devops", re: /\b(devops|sre|site reliability|infrastructure|platform\s+engineer|cloud\s+engineer|network\s+engineer)\b/i },
  { role: "hardware", re: /\b(hardware|firmware|embedded|fpga|asic|robotics|silicon|semiconductor|rtl|vlsi|chip\s+design|electrical\s+engineer)\b/i },
  { role: "quant", re: /\b(quant|quantitative)\b/i },
  { role: "product", re: /\b(product\s*manager|technical\s+product|\bpm\b|product\s+intern)\b/i },
  {
    role: "research",
    re: /\b(research\s+(engineer|scientist)|(?:ai|ml|machine learning|computer vision|quantitative|software)\s+research)\b/i,
  },
  { role: "software", re: /\b(software|swe|sde|developer|programming|algorithm\s+engineer|full[\s-]?stack|forward\s+deployed\s+engineer)\b/i },
];

function rolesIn(text: string): Set<JobRole> {
  const found = new Set<JobRole>();
  for (const { role, re } of ROLE_PATTERNS) {
    if (re.test(text)) found.add(role);
  }
  // Technical research titles often state the domain separately, e.g.
  // "Research Intern — Machine Learning".
  if (
    /\bresearch\b/i.test(text) &&
    [...found].some((role) => ["software", "data", "ml", "hardware", "quant"].includes(role))
  ) {
    found.add("research");
  }
  return found;
}

export function extractRoles(title: string, description = ""): JobRole[] {
  const fromTitle = rolesIn(title);
  if (fromTitle.size > 0) return ROLE_OPTIONS.filter((r) => fromTitle.has(r));

  // Descriptions are only a fallback for generic technical titles. Never union
  // every discipline mentioned in a job description into an otherwise clear title.
  const fromDescription = rolesIn(description);
  return ROLE_OPTIONS.filter((r) => fromDescription.has(r));
}

// Prefer full names + city landmarks; state abbrevs after a comma so "CA" ≠ Canada.
const US_RE =
  /\b(united states|\bUSA\b|\bU\.S\.A\.?\b|,?\s*U\.?S\.?A?\.?\b|california|new york|texas|washington|massachusetts|illinois|georgia|florida|colorado|arizona|oregon|virginia|north carolina|new jersey|pennsylvania|ohio|michigan|seattle|san francisco|foster city|bay area|nyc|los angeles|austin|boston|chicago|denver|atlanta|sf\b|palo alto|mountain view|sunnyvale|menlo park|cupertino|redmond|bellevue|spacex\s*site|hawthorne|boca chica|cape canaveral|kennedy space|starbase)\b|, (AL|AK|AZ|AR|CA|CO|CT|DE|FL|GA|HI|IA|ID|IL|IN|KS|KY|LA|MA|MD|ME|MI|MN|MO|MS|MT|NC|ND|NE|NH|NJ|NM|NV|NY|OH|OK|OR|PA|RI|SC|SD|TN|TX|UT|VA|VT|WA|WI|WV)\b/i;
const CANADA_RE =
  /\b(canada|toronto|vancouver|montreal|ottawa|calgary|waterloo|ontario|quebec|british columbia|\bB\.?C\.?\b|, ON\b|, QC\b|, AB\b|, BC\b|, MB\b|, SK\b|, NS\b|, NB\b)\b/i;
const EUROPE_RE =
  /\b(united kingdom|\bUK\b|england|london|ireland|dublin|europe|european|germany|berlin|munich|france|paris|netherlands|amsterdam|sweden|stockholm|switzerland|zurich|spain|madrid|barcelona|italy|milan|denmark|copenhagen|finland|helsinki|norway|oslo|portugal|lisbon|austria|vienna|belgium|brussels|poland|warsaw)\b/i;

export function extractRegions(
  locations: string[],
  isRemote: boolean,
): JobRegion[] {
  const found = new Set<JobRegion>();
  const blob = locations.join(" ");
  if (isRemote || /\bremote\b/i.test(blob)) found.add("remote");
  if (US_RE.test(blob)) found.add("us");
  if (CANADA_RE.test(blob)) found.add("canada");
  if (EUROPE_RE.test(blob)) found.add("europe");
  if (locations.length > 0 && found.size === (found.has("remote") ? 1 : 0)) {
    // Had locations but nothing matched a known country — other
    if (!found.has("remote") || locations.some((l) => !/\bremote\b/i.test(l))) {
      found.add("other");
    }
  } else if (locations.length > 0) {
    const nonRemote = locations.filter((l) => !/\bremote\b/i.test(l));
    if (
      nonRemote.length > 0 &&
      !US_RE.test(nonRemote.join(" ")) &&
      !CANADA_RE.test(nonRemote.join(" ")) &&
      !EUROPE_RE.test(nonRemote.join(" "))
    ) {
      found.add("other");
    }
  }
  return REGION_OPTIONS.filter((r) => found.has(r));
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
  let text = stripTags(html);
  text = decodeEntities(text);
  text = stripTags(text);
  text = decodeEntities(text);
  text = text.replace(/\s+/g, " ").trim();
  if (!text) return null;
  return text.length > max ? `${text.slice(0, max - 1)}…` : text;
}
