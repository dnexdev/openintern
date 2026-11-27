// Builds the ingest parity scenario: board contents for a fake fetcher
// (boards.json.gz) and two runs of days (scenarios.json). Deterministic.
// Usage (repo root):
//   packages/ingest/node_modules/.bin/tsx internal/ingest/testdata/gen-scenario.mts
import fs from "node:fs";
import path from "node:path";
import { gzipSync } from "node:zlib";
import { loadCompanyYamlFiles } from "../../../packages/ingest/src/sync-companies.ts";

const here = path.dirname(new URL(import.meta.url).pathname);
const dump = JSON.parse(fs.readFileSync(path.join(here, "../../../.cache/dump/jobs.json"), "utf8"));
const registry = [...new Map((await loadCompanyYamlFiles()).map((c) => [c.slug, c])).values()];

type J = { externalId: string; title: string; locations: string[]; applyUrl: string; excerpt: string | null; postedAt: string | null; description: string };
const boards: Record<string, J[]> = {};
const key = (ats: string, token: string) => `${ats}\t${token}`;
const bySlug = new Map<string, number[]>();
dump.jobs.forEach((j: any, i: number) => bySlug.set(j.company.slug, [...(bySlug.get(j.company.slug) ?? []), i]));

registry.forEach((c, k) => {
  const list: J[] = [];
  for (const i of bySlug.get(c.slug) ?? []) {
    const j = dump.jobs[i];
    list.push({
      externalId: `ext-${i}`,
      title: j.title,
      locations: j.locations ?? [],
      applyUrl: j.apply_url,
      excerpt: i % 3 === 0 ? null : i % 3 === 1 ? `<p>${j.excerpt ?? ""}</p>` : j.excerpt,
      postedAt: i % 11 === 0 ? null : i % 97 === 1 ? "not a date" : j.posted_at,
      description: j.excerpt ?? "",
    });
  }
  if (k % 5 === 0) {
    list.push(
      { externalId: `syn-${k}-senior`, title: "Senior Software Engineer", locations: ["Remote"], applyUrl: `https://example.com/${k}/1`, excerpt: null, postedAt: "2026-10-30T00:00:00Z", description: "Build things. 5+ years." },
      { externalId: `syn-${k}-mkt`, title: "Marketing Intern", locations: ["New York, NY"], applyUrl: `https://example.com/${k}/2`, excerpt: null, postedAt: null, description: "Social media internship." },
      { externalId: `syn-${k}-nodesc`, title: "Software Engineering Intern", locations: ["Toronto, ON"], applyUrl: `https://example.com/${k}/3`, excerpt: null, postedAt: "2026-10-29T08:00:00.123Z", description: "  " },
      { externalId: `syn-${k}-swe`, title: "Software Engineer Intern (Summer 2027)", locations: ["San Francisco, CA", "Remote - US"], applyUrl: `https://evil.example/apply/${k}`, excerpt: "Write <b>code</b> &amp; ship 🚀", postedAt: "2026-10-28T10:00:00Z", description: "Software engineering internship, Python, React. 12 weeks." },
      { externalId: `syn-${k}-http`, title: "Machine Learning Intern - Fall 2026", locations: ["London, UK"], applyUrl: `http://example.com/${k}/5`, excerpt: null, postedAt: "2026-06-01T00:00:00Z", description: "ML research internship, PyTorch." },
    );
  }
  if (list.length === 0 && k % 7 === 2) {
    list.push({ externalId: `syn-${k}-only-senior`, title: "Staff Engineer", locations: [], applyUrl: `https://example.com/${k}/9`, excerpt: null, postedAt: null, description: "Lead." });
  }
  if (list.length) boards[key(c.ats, c.board_token)] = list;
  for (const [t, extra] of (c.extra_board_tokens ?? []).entries()) {
    boards[key(c.ats, extra)] = [
      ...(list[0] ? [{ ...list[0], title: list[0].title + " (dup board)" }] : []),
      { externalId: `syn-${k}-extra-${t}`, title: "Hardware Engineering Intern", locations: ["Austin, TX"], applyUrl: `https://example.com/${k}/x${t}`, excerpt: null, postedAt: "2026-10-31T00:00:00Z", description: "FPGA and embedded internship." },
    ];
  }
});

const keys = registry.map((c) => key(c.ats, c.board_token));
const pick = (pred: (k: number) => boolean) => keys.filter((_, k) => pred(k));
const allExt = Object.values(boards).flat().map((j) => j.externalId);
const hash = (s: string) => [...s].reduce((h, ch) => (h * 31 + ch.charCodeAt(0)) >>> 0, 7);
const tesla = registry.find((c) => c.ats === "tesla");

type Day = { now: string; env?: Record<string, string>; skipSync?: boolean; errors?: Record<string, { kind?: string; message: string; status?: number }>; empty?: string[]; drop?: string[] };
const refused = Object.fromEntries(pick((k) => k % 41 === 3).map((k) => [k, { kind: "refused", message: "HTTP 403", status: 403 }]));
// Boards that list jobs on day 1 and fail from then on: auto-disabled, and
// their jobs age out through the last-seen sweep.
const late = Object.fromEntries(pick((k) => k % 43 === 10).map((k) => [k, { kind: "network", message: "connect ECONNREFUSED" }]));
const flaky = (d: number) => Object.fromEntries(pick((k) => k % 59 === (d % 5)).map((k) => [k, { message: `fetch failed ${"x".repeat(d * 700)}` }]));
const empties = pick((k) => k % 29 === 5);
const drop = (d: number) => allExt.filter((e) => hash(e + d) % 10 === 0);
const day = (n: string, extra: Partial<Day> = {}): Day => ({ now: n, ...extra });

const b: Day[] = [
  day("2026-11-01T12:00:00.000Z", { errors: { ...refused, ...flaky(1) } }),
  day("2026-11-02T12:00:00.000Z", { errors: { ...refused, ...late, ...flaky(2) }, empty: empties, drop: drop(2) }),
  day("2026-11-03T12:00:00.250Z", {
    errors: { ...refused, ...late, ...(tesla ? { [key("tesla", tesla.board_token)]: { kind: "manual-expired", message: "data/manual/tesla.json is 15 days old" } } : {}) },
    empty: empties, skipSync: true,
  }),
  day("2026-11-04T12:00:00.000Z", { errors: { ...refused, ...late }, empty: empties, drop: drop(4) }),
  day("2026-11-05T12:00:00.000Z", { errors: { ...refused, ...late }, env: { OPENINTERN_ONLY_SLUGS: ` ${registry.filter((_, k) => k % 13 === 3).map((c) => c.slug).join(" , ")},,` } }),
  day("2026-11-06T12:00:00.000Z", { errors: { ...refused, ...late }, env: { OPENINTERN_APPLY_HOSTS: "enforce" } }),
  day("2026-11-07T12:00:00.000Z", { errors: { ...refused, ...late, ...flaky(7) } }),
  day("2026-11-08T12:00:00.000Z", { errors: { ...refused, ...late } }),
  day("2026-11-14T12:00:00.000Z", { errors: { ...refused, ...late }, drop: drop(9) }),
  day("2026-11-16T12:00:00.000Z", { errors: { ...refused, ...late } }),
  day("2027-01-20T12:00:00.000Z", { errors: refused, env: { OPENINTERN_APPLY_HOSTS: "enforce" } }),
];
const a: Day[] = [
  day("2026-11-14T12:00:00.000Z", { errors: { ...refused, ...flaky(1) } }),
  day("2026-11-15T12:00:00.000Z", { errors: refused, drop: drop(3), empty: empties }),
];
fs.writeFileSync(path.join(here, "boards.json.gz"), gzipSync(JSON.stringify(boards), { level: 9 }));
fs.writeFileSync(path.join(here, "scenarios.json"), JSON.stringify({ empty: b, "state-real": a }, null, 1) + "\n");
console.log(`${Object.keys(boards).length} boards, ${allExt.length} postings`);
