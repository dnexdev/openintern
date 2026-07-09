/**
 * Heuristic internship / early-career classifier for tech roles.
 * Keep intern, co-op, new grad; drop senior/staff/principal noise.
 */

const INCLUDE =
  /\b(intern|internship|co-?op|coop|new\s*grad|university|campus|student|graduate\s*program|apprentice)\b/i;

const EXCLUDE =
  /\b(senior|staff|principal|director|manager|lead|head of|vp\b|vice president|chief|architect)\b/i;

const TECH_HINT =
  /\b(software|engineer|engineering|developer|swe|sde|frontend|backend|full[\s-]?stack|data|ml|machine learning|ai|artificial intelligence|research|security|infra|infrastructure|devops|sre|platform|mobile|ios|android|firmware|hardware|embedded|quant|quantitative|product\s*manager|pm\b|design\s*engineer|site reliability)\b/i;

export function isTechInternship(title: string, description = ""): boolean {
  const text = `${title} ${description}`.trim();
  if (!INCLUDE.test(text)) return false;
  // Allow "Senior Intern" style edge cases only if clearly internship-titled
  if (EXCLUDE.test(title) && !/\b(intern|internship|co-?op)\b/i.test(title)) {
    return false;
  }
  // Prefer tech-shaped titles; still keep generic "Software Intern" etc.
  if (TECH_HINT.test(text) || /\b(intern|co-?op)\b/i.test(title)) {
    return true;
  }
  return false;
}

export function looksRemote(locations: string[], title = ""): boolean {
  const blob = `${locations.join(" ")} ${title}`.toLowerCase();
  return /\bremote\b|\bwfh\b|work from home|anywhere/.test(blob);
}

export function excerptFromHtml(html: string | undefined | null, max = 400): string | null {
  if (!html) return null;
  const text = html
    .replace(/<script[\s\S]*?<\/script>/gi, " ")
    .replace(/<style[\s\S]*?<\/style>/gi, " ")
    .replace(/<[^>]+>/g, " ")
    .replace(/&nbsp;/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/\s+/g, " ")
    .trim();
  if (!text) return null;
  return text.length > max ? `${text.slice(0, max - 1)}…` : text;
}
