import { COMPANY_ALIASES, JOB_ALIASES } from "./aliases.generated";

export { COMPANY_ALIASES, JOB_ALIASES };

/** The current slug for a company, following merges and renames. */
export function canonicalCompanySlug(slug: string): string {
  return COMPANY_ALIASES.get(slug) ?? slug;
}

/** The job that replaced a retired job id, if any. */
export function jobAlias(id: string): string | null {
  return JOB_ALIASES.get(id.toLowerCase()) ?? null;
}

/** The replacement for a retired job whose id starts with this 8-hex prefix (job slugs carry only the prefix). */
export function jobAliasByPrefix(prefix8: string): string | null {
  const p = prefix8.toLowerCase();
  for (const [oldId, newId] of JOB_ALIASES) if (oldId.replace(/-/g, "").startsWith(p)) return newId;
  return null;
}
