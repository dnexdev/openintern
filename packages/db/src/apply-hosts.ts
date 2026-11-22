import { APPLY_HOSTS } from "./apply-hosts.generated";

export { APPLY_HOSTS };

/** [host, path prefix] pairs a company's apply links may use. */
export type ApplyHostTable = Readonly<Record<string, readonly (readonly [string, string])[]>>;

export type ApplyCheck =
  | { ok: true }
  | { ok: false; reason: "not-https" | "malformed" | "credentials-or-port" | "unknown-company" | "off-list"; host: string | null };

/**
 * Whether an apply URL may be shown for a company: https, no credentials or
 * port, and a host plus path prefix on the company's allowlist. Hosts compare
 * exactly; prefixes compare case-insensitively on whole path segments.
 */
export function checkApplyUrl(companySlug: string, url: string, table: ApplyHostTable = APPLY_HOSTS): ApplyCheck {
  let u: URL;
  try {
    u = new URL(url);
  } catch {
    return { ok: false, reason: "malformed", host: null };
  }
  const host = u.hostname.toLowerCase();
  if (u.protocol !== "https:") return { ok: false, reason: "not-https", host };
  if (u.username || u.password || u.port) return { ok: false, reason: "credentials-or-port", host };
  const allowed = table[companySlug];
  if (!allowed) return { ok: false, reason: "unknown-company", host };
  const path = `${u.pathname.toLowerCase().replace(/\/+$/, "")}/`;
  for (const [h, prefix] of allowed) {
    if (h === host && path.startsWith(prefix.toLowerCase())) return { ok: true };
  }
  return { ok: false, reason: "off-list", host };
}

/**
 * off: no checks. warn (default, shadow mode): violations are logged, nothing
 * is hidden. enforce: violating links are not ingested, dumped or shown.
 */
export type ApplyHostMode = "off" | "warn" | "enforce";

export function applyHostMode(): ApplyHostMode {
  const v = process.env.OPENINTERN_APPLY_HOSTS?.trim();
  return v === "off" || v === "enforce" ? v : "warn";
}

const warned = new Set<string>();

/**
 * The gate every publishing path calls. In warn mode it logs each violation
 * once per process and returns true; in enforce mode it returns false.
 */
export function applyUrlAllowed(companySlug: string, url: string, mode = applyHostMode()): boolean {
  if (mode === "off") return true;
  const check = checkApplyUrl(companySlug, url);
  if (check.ok) return true;
  const key = `${companySlug} ${url}`;
  if (!warned.has(key)) {
    warned.add(key);
    console.warn(`[apply-hosts] ${mode === "enforce" ? "blocked" : "would block"} ${companySlug}: ${check.reason} ${url}`);
  }
  return mode !== "enforce";
}
