import {
  encodeWorkdayToken,
  parseWorkdayUrl,
  workdayCareersUrl,
} from "./workday.js";
import { stripHtml } from "./strip-html.js";

/**
 * Map public Apply / careers URLs to supported ATS board tokens.
 * Greenhouse embed job IDs and generic career pages return null.
 */

export type SupportedAts =
  | "greenhouse"
  | "lever"
  | "ashby"
  | "workable"
  | "smartrecruiters"
  | "recruitee"
  | "rippling"
  | "bamboohr"
  | "workday";

export type ParsedAtsBoard = {
  ats: SupportedAts;
  boardToken: string;
  careersUrl: string;
};

const SKIP_PATH_TOKENS = new Set([
  "embed",
  "job_app",
  "jobs",
  "o",
  "application",
  "posting",
  "careers",
  "www",
]);

function boardRoot(ats: SupportedAts, token: string): string {
  switch (ats) {
    case "greenhouse":
      return `https://job-boards.greenhouse.io/${token}`;
    case "lever":
      return `https://jobs.lever.co/${token}`;
    case "ashby":
      return `https://jobs.ashbyhq.com/${token}`;
    case "workable":
      return `https://apply.workable.com/${token}`;
    case "smartrecruiters":
      return `https://jobs.smartrecruiters.com/${token}`;
    case "recruitee":
      return `https://${token}.recruitee.com`;
    case "rippling":
      return `https://ats.rippling.com/${token}`;
    case "bamboohr":
      return `https://${token}.bamboohr.com/careers`;
    case "workday": {
      const parts = token.split("|");
      if (parts.length === 3) {
        return workdayCareersUrl({
          tenant: parts[0]!,
          datacenter: parts[1]!,
          site: parts[2]!,
        });
      }
      return `https://www.myworkdayjobs.com/${encodeURIComponent(token)}`;
    }
  }
}

function cleanToken(raw: string): string | null {
  const t = decodeURIComponent(raw).trim().replace(/\/+$/, "");
  if (!t || SKIP_PATH_TOKENS.has(t.toLowerCase())) return null;
  // SmartRecruiters company ids are often TitleCase; keep original case for SR
  return t;
}

/**
 * Parse a single Apply URL. Returns null when the host is unsupported
 * or the path is a job-id embed rather than a board token.
 */
export function parseAtsFromApplyUrl(rawUrl: string): ParsedAtsBoard | null {
  let url: URL;
  try {
    url = new URL(rawUrl.trim());
  } catch {
    return null;
  }

  const host = url.hostname.toLowerCase().replace(/^www\./, "");
  const parts = url.pathname.split("/").filter(Boolean);

  // Greenhouse board paths
  if (
    host === "job-boards.greenhouse.io" ||
    host === "boards.greenhouse.io" ||
    host === "boards-api.greenhouse.io"
  ) {
    // embed/job_app?token=JOB_ID — not a board token
    if (parts[0]?.toLowerCase() === "embed") return null;
    const token = cleanToken(parts[0] ?? "");
    if (!token) return null;
    return {
      ats: "greenhouse",
      boardToken: token.toLowerCase(),
      careersUrl: boardRoot("greenhouse", token.toLowerCase()),
    };
  }

  if (host === "jobs.ashbyhq.com") {
    const token = cleanToken(parts[0] ?? "");
    if (!token) return null;
    const boardToken = token.toLowerCase();
    return {
      ats: "ashby",
      boardToken,
      careersUrl: boardRoot("ashby", boardToken),
    };
  }

  if (host === "jobs.lever.co") {
    const token = cleanToken(parts[0] ?? "");
    if (!token) return null;
    return {
      ats: "lever",
      boardToken: token.toLowerCase(),
      careersUrl: boardRoot("lever", token.toLowerCase()),
    };
  }

  if (host === "apply.workable.com") {
    const token = cleanToken(parts[0] ?? "");
    if (!token) return null;
    return {
      ats: "workable",
      boardToken: token.toLowerCase(),
      careersUrl: boardRoot("workable", token.toLowerCase()),
    };
  }

  if (host === "jobs.smartrecruiters.com") {
    const token = cleanToken(parts[0] ?? "");
    if (!token) return null;
    // SmartRecruiters tokens are often case-sensitive
    return {
      ats: "smartrecruiters",
      boardToken: token,
      careersUrl: boardRoot("smartrecruiters", token),
    };
  }

  if (host === "ats.rippling.com") {
    const token = cleanToken(parts[0] ?? "");
    if (!token) return null;
    return {
      ats: "rippling",
      boardToken: token.toLowerCase(),
      careersUrl: boardRoot("rippling", token.toLowerCase()),
    };
  }

  const recruitee = host.match(/^([a-z0-9-]+)\.recruitee\.com$/i);
  if (recruitee) {
    const token = cleanToken(recruitee[1]!);
    if (!token) return null;
    return {
      ats: "recruitee",
      boardToken: token.toLowerCase(),
      careersUrl: boardRoot("recruitee", token.toLowerCase()),
    };
  }

  const bamboo = host.match(/^([a-z0-9-]+)\.bamboohr\.com$/i);
  if (bamboo) {
    const token = cleanToken(bamboo[1]!);
    if (!token) return null;
    return {
      ats: "bamboohr",
      boardToken: token.toLowerCase(),
      careersUrl: boardRoot("bamboohr", token.toLowerCase()),
    };
  }

  // Workday CXS: tenant.wdN.myworkdayjobs.com/{site}/...
  const workday = parseWorkdayUrl(rawUrl);
  if (workday) {
    const boardToken = encodeWorkdayToken(workday);
    return {
      ats: "workday",
      boardToken,
      careersUrl: workdayCareersUrl(workday),
    };
  }

  return null;
}

export function tokenKey(ats: string, boardToken: string): string {
  // SmartRecruiters + Workday site paths are case-sensitive.
  if (ats === "smartrecruiters" || ats === "workday") {
    return `${ats}:${boardToken}`;
  }
  return `${ats}:${boardToken.toLowerCase()}`;
}

export type ExtractedAtsHit = {
  companyName: string;
  applyUrl: string;
  ats: SupportedAts;
  boardToken: string;
  careersUrl: string;
};

function companyFromCell(raw: string): string | null {
  const cell = stripHtml(
    raw.replace(/\*\*/g, "").replace(/\[([^\]]+)\]\([^)]+\)/g, "$1"),
  );
  if (!cell || cell === "↳") return null;
  if (cell.length < 2 || cell.length > 80) return null;
  if (/^(company|position|role|location|salary|age)$/i.test(cell)) return null;
  if (!/[a-zA-Z]/.test(cell)) return null;
  return cell;
}

/**
 * Extract company + Apply URL pairs from markdown/HTML tables, keeping only
 * rows whose Apply link maps to a supported ATS board.
 */
export function extractAtsBoardsFromMarkdown(md: string): {
  hits: ExtractedAtsHit[];
  applyLinkCount: number;
  unsupportedApplyCount: number;
  unsupportedHosts: Record<string, number>;
} {
  const hits: ExtractedAtsHit[] = [];
  let applyLinkCount = 0;
  let unsupportedApplyCount = 0;
  const unsupportedHosts: Record<string, number> = {};

  const recordApply = (companyName: string, applyUrlRaw: string) => {
    applyLinkCount += 1;
    const applyUrl = applyUrlRaw.replace(/&amp;/g, "&");
    const parsed = parseAtsFromApplyUrl(applyUrl);
    if (!parsed) {
      unsupportedApplyCount += 1;
      try {
        const host = new URL(applyUrl).hostname.toLowerCase().replace(/^www\./, "");
        unsupportedHosts[host] = (unsupportedHosts[host] ?? 0) + 1;
      } catch {
        unsupportedHosts["(invalid)"] = (unsupportedHosts["(invalid)"] ?? 0) + 1;
      }
      return;
    }
    hits.push({
      companyName,
      applyUrl,
      ats: parsed.ats,
      boardToken: parsed.boardToken,
      careersUrl: parsed.careersUrl,
    });
  };

  // HTML tables (SimplifyJobs README): <tr>… company in first <td>, Apply as
  // <a href="..."><img alt="Apply"></a>
  let lastCompanyHtml: string | null = null;
  for (const tr of md.matchAll(/<tr[^>]*>([\s\S]*?)<\/tr>/gi)) {
    const row = tr[1]!;
    const tds = [...row.matchAll(/<td[^>]*>([\s\S]*?)<\/td>/gi)].map((m) => m[1]!);
    if (tds.length < 2) continue;

    const companyCell = companyFromCell(tds[0]!);
    if (companyCell) lastCompanyHtml = companyCell;
    const companyName = companyCell ?? lastCompanyHtml;
    if (!companyName) continue;

    // Prefer links whose img alt is Apply; fall back to any non-simplify.jobs href in the Application cell
    const applyCell = tds.find((td) => /alt=["']Apply["']/i.test(td)) ?? tds[tds.length - 2] ?? "";
    const applyHrefs = [
      ...applyCell.matchAll(
        /<a\s+[^>]*href=["'](https?:[^"']+)["'][^>]*>\s*<img[^>]*alt=["']Apply["']/gi,
      ),
    ].map((m) => m[1]!);

    if (applyHrefs.length === 0) {
      for (const m of applyCell.matchAll(/<a\s+[^>]*href=["'](https?:[^"']+)["']/gi)) {
        const href = m[1]!;
        if (/simplify\.jobs/i.test(href)) continue;
        applyHrefs.push(href);
      }
    }

    for (const href of applyHrefs) {
      recordApply(companyName, href);
    }
  }

  // Markdown pipe tables with [Apply](url) (tests / alternate formats)
  let lastCompanyMd: string | null = null;
  for (const line of md.split("\n")) {
    if (!line.includes("|") || !/\[Apply\]\(/i.test(line)) continue;
    if (/^\s*\|?\s*:?-{3,}/.test(line)) continue;
    if (/<td[\s>]/i.test(line)) continue; // already handled as HTML

    const cells = line.split("|").map((c) => c.trim()).filter(Boolean);
    if (cells.length < 2) continue;

    const companyCell = companyFromCell(cells[0]!);
    if (companyCell) lastCompanyMd = companyCell;
    const companyName = companyCell ?? lastCompanyMd;
    if (!companyName) continue;

    for (const m of line.matchAll(/\[Apply\]\((https?:[^)\s]+)\)/gi)) {
      recordApply(companyName, m[1]!);
    }
  }

  return { hits, applyLinkCount, unsupportedApplyCount, unsupportedHosts };
}
