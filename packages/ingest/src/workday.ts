/**
 * Workday public CXS board token encoding.
 * Format: tenant|wdN|site  e.g. nvidia|wd5|NVIDIAExternalCareerSite
 */

export type WorkdayBoard = {
  tenant: string;
  datacenter: string;
  site: string;
};

export function parseWorkdayToken(boardToken: string): WorkdayBoard | null {
  const parts = boardToken.split("|").map((p) => p.trim()).filter(Boolean);
  if (parts.length !== 3) return null;
  const [tenant, datacenter, site] = parts as [string, string, string];
  if (!/^wd\d+$/i.test(datacenter)) return null;
  if (!tenant || !site) return null;
  return {
    tenant,
    datacenter: datacenter.toLowerCase(),
    site,
  };
}

export function encodeWorkdayToken(board: WorkdayBoard): string {
  return `${board.tenant}|${board.datacenter.toLowerCase()}|${board.site}`;
}

export function workdayJobsUrl(board: WorkdayBoard): string {
  return `https://${board.tenant}.${board.datacenter}.myworkdayjobs.com/wday/cxs/${encodeURIComponent(board.tenant)}/${encodeURIComponent(board.site)}/jobs`;
}

export function workdayCareersUrl(board: WorkdayBoard): string {
  return `https://${board.tenant}.${board.datacenter}.myworkdayjobs.com/${board.site}`;
}

export function workdayDetailUrl(board: WorkdayBoard, externalPath: string): string {
  const path = externalPath.startsWith("/") ? externalPath : `/${externalPath}`;
  return `https://${board.tenant}.${board.datacenter}.myworkdayjobs.com/wday/cxs/${encodeURIComponent(board.tenant)}/${encodeURIComponent(board.site)}${path}`;
}

export function workdayApplyUrl(board: WorkdayBoard, externalPath: string): string {
  const path = externalPath.startsWith("/") ? externalPath : `/${externalPath}`;
  return `https://${board.tenant}.${board.datacenter}.myworkdayjobs.com/${board.site}${path}`;
}

/**
 * Parse a public Workday careers / job URL into a board token.
 * Examples:
 *   https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite
 *   https://nvidia.wd5.myworkdayjobs.com/en-US/NVIDIAExternalCareerSite/job/...
 *   https://selinc.wd1.myworkdayjobs.com/SEL/job/...
 */
export function parseWorkdayUrl(rawUrl: string): WorkdayBoard | null {
  let url: URL;
  try {
    url = new URL(rawUrl.trim());
  } catch {
    return null;
  }
  const host = url.hostname.toLowerCase();
  const m = host.match(/^([a-z0-9-]+)\.(wd\d+)\.myworkdayjobs\.com$/i);
  if (!m) return null;
  const tenant = m[1]!;
  const datacenter = m[2]!.toLowerCase();

  const parts = url.pathname.split("/").filter(Boolean);
  // Drop locale prefixes like en-US
  const filtered = parts.filter((p) => !/^[a-z]{2}(-[A-Z]{2})?$/i.test(p));
  if (filtered.length === 0) return null;

  // Skip CXS API paths
  if (filtered[0]?.toLowerCase() === "wday") return null;

  const site = filtered[0]!;
  if (!site || /^(job|jobs)$/i.test(site)) return null;

  return { tenant, datacenter, site };
}
