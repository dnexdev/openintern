/**
 * Eightfold PCSX public search (Microsoft careers and similar tenants).
 *
 * board_token:
 *   - `microsoft` → domain=microsoft.com, query=intern
 *   - `microsoft|ai` → same domain, AI/Research-oriented query + title filter
 *   - `domain.com|query` → custom domain + search query
 */

import type { NormalizedJob } from "./ats.js";

const UA = "OpenIntern/0.1 (+https://github.com/dnexdev/openintern)";
const PAGE_SIZE = 10;
const MAX_PAGES = 40;

export type EightfoldPosition = {
  id?: number | string;
  displayJobId?: string;
  name?: string;
  title?: string;
  locations?: string[];
  standardizedLocations?: string[];
  postedTs?: number;
  creationTs?: number;
  department?: string;
  positionUrl?: string;
};

export type EightfoldSearchData = {
  positions?: EightfoldPosition[];
  count?: number;
};

export type EightfoldSearchResponse = {
  status?: number;
  data?: EightfoldSearchData;
};

export type EightfoldBoard = {
  domain: string;
  query: string;
  /** Extra title/dept regex filter (microsoft-ai). */
  titleFilter?: RegExp;
  applyOrigin: string;
};

const AI_TITLE =
  /\b(ai|a\.i\.|ml|machine learning|llm|deep learning|research|coreai|mai|openai|generative)\b/i;

export function parseEightfoldToken(boardToken: string): EightfoldBoard {
  const parts = boardToken.split("|").map((p) => p.trim()).filter(Boolean);
  const head = (parts[0] || "microsoft").toLowerCase();
  const filterKey = (parts[1] || "").toLowerCase();

  if (head === "microsoft" || head === "microsoft.com") {
    const applyOrigin = "https://apply.careers.microsoft.com";
    if (filterKey === "ai" || filterKey === "research") {
      return {
        domain: "microsoft.com",
        query: "AI Intern",
        titleFilter: AI_TITLE,
        applyOrigin,
      };
    }
    return {
      domain: "microsoft.com",
      query: parts[1] || "intern",
      applyOrigin,
    };
  }

  const domain = head.includes(".") ? head : `${head}.com`;
  return {
    domain,
    query: parts[1] || "intern",
    applyOrigin: `https://apply.careers.${domain.replace(/\.com$/i, ".com")}`,
  };
}

export function eightfoldSearchUrl(board: EightfoldBoard, start: number): string {
  const u = new URL("https://apply.careers.microsoft.com/api/pcsx/search");
  // Microsoft hosts PCSX; other domains may need a different host later.
  if (board.domain !== "microsoft.com") {
    u.host = `apply.careers.${board.domain}`;
    u.pathname = "/api/pcsx/search";
  }
  u.searchParams.set("domain", board.domain);
  u.searchParams.set("query", board.query);
  u.searchParams.set("num", String(PAGE_SIZE));
  u.searchParams.set("start", String(start));
  return u.toString();
}

export function mapEightfoldPositions(
  positions: EightfoldPosition[],
  board: EightfoldBoard,
): NormalizedJob[] {
  const jobs: NormalizedJob[] = [];
  for (const p of positions) {
    const title = (p.name || p.title || "").trim();
    if (!title) continue;
    if (board.titleFilter && !board.titleFilter.test(title) && !board.titleFilter.test(p.department || "")) {
      continue;
    }
    const id = String(p.id ?? p.displayJobId ?? "").trim();
    if (!id) continue;
    const path = p.positionUrl?.startsWith("/")
      ? p.positionUrl
      : `/careers/job/${id}`;
    const applyUrl = `${board.applyOrigin}${path}`;
    const locations = [
      ...(p.standardizedLocations ?? []),
      ...(p.locations ?? []),
    ].filter((l, i, arr) => l && arr.indexOf(l) === i);
    const ts = p.postedTs || p.creationTs;
    jobs.push({
      externalId: id,
      title,
      locations,
      applyUrl,
      excerpt: p.department ? p.department.slice(0, 400) : null,
      postedAt: ts ? new Date(ts * 1000) : null,
      description: p.department || "",
    });
  }
  return jobs;
}

export async function fetchEightfold(boardToken: string): Promise<NormalizedJob[]> {
  const board = parseEightfoldToken(boardToken);
  const all: NormalizedJob[] = [];
  const seen = new Set<string>();
  let total = Infinity;

  for (let page = 0; page < MAX_PAGES; page++) {
    const start = page * PAGE_SIZE;
    if (start >= total) break;
    const url = eightfoldSearchUrl(board, start);
    const res = await fetch(url, {
      headers: {
        Accept: "application/json",
        "User-Agent": UA,
        Referer: `${board.applyOrigin}/careers`,
      },
    });
    if (!res.ok) {
      throw new Error(`Eightfold HTTP ${res.status} for ${url}`);
    }
    const body = (await res.json()) as EightfoldSearchResponse;
    const positions = body.data?.positions ?? [];
    if (typeof body.data?.count === "number") total = body.data.count;
    if (positions.length === 0) break;
    for (const job of mapEightfoldPositions(positions, board)) {
      if (seen.has(job.externalId)) continue;
      seen.add(job.externalId);
      all.push(job);
    }
    if (positions.length < PAGE_SIZE) break;
  }

  return all;
}

export async function probeEightfold(
  boardToken: string,
): Promise<{ ok: boolean; status: number; count?: number; error?: string }> {
  try {
    const board = parseEightfoldToken(boardToken);
    const url = eightfoldSearchUrl(board, 0);
    const res = await fetch(url, {
      headers: {
        Accept: "application/json",
        "User-Agent": UA,
        Referer: `${board.applyOrigin}/careers`,
      },
    });
    if (!res.ok) return { ok: false, status: res.status, error: `HTTP ${res.status}` };
    const body = (await res.json()) as EightfoldSearchResponse;
    const count = body.data?.count ?? body.data?.positions?.length ?? 0;
    return { ok: count > 0, status: res.status, count };
  } catch (err) {
    return { ok: false, status: 0, error: err instanceof Error ? err.message : String(err) };
  }
}
