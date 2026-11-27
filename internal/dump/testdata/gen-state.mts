// Builds a realistic state from the cached public dump plus synthetic runs,
// written by the TypeScript writeState. Usage (repo root):
//   packages/ingest/node_modules/.bin/tsx internal/dump/testdata/gen-state.mts .cache/dump/jobs.json <out-dir>
import fs from "node:fs";
import { writeState, pgTimestamp, type State } from "../../../packages/ingest/src/state.ts";
import { normalizeTitle, jobFingerprint, roleFamilyId } from "../../../packages/ingest/src/normalize-title.ts";
import { DELISTED_JOBS } from "../../../packages/db/src/delisted.ts";

const [dumpPath, out] = process.argv.slice(2);
const dump = JSON.parse(fs.readFileSync(dumpPath, "utf8"));
let seed = 42;
const rnd = () => ((seed = (seed * 1103515245 + 12345) % 2147483648) / 2147483648);
const hex = (n: number) => Array.from({ length: n }, () => "0123456789abcdef"[Math.floor(rnd() * 16)]).join("");
const uuid = () => `${hex(8)}-${hex(4)}-4${hex(3)}-a${hex(3)}-${hex(12)}`;
// Timestamp spellings found in real state: pgTimestamp and row_to_json.
const ts = (iso: string, i: number) => {
  const d = new Date(iso);
  if (i % 3 === 0) return pgTimestamp(d);
  if (i % 3 === 1) return iso.replace("Z", "+00:00").replace(/\.(\d{3})/, ".$1" + String(i % 1000).padStart(3, "0"));
  return iso.replace(/\.\d{3}Z$/, "+00:00");
};
const state: State = { companies: new Map(), jobs: new Map(), runs: [] };
const bySlug = new Map<string, any>();
let ci = 0;
for (const j of dump.jobs) {
  if (bySlug.has(j.company.slug)) continue;
  const c = {
    id: uuid(), name: j.company.name, slug: j.company.slug, ats: j.company.ats, board_token: j.company.slug,
    careers_url: ci % 4 === 0 ? null : `https://${j.company.slug}.example/careers`,
    website_url: ci % 5 === 0 ? null : `https://${j.company.slug}.example`,
    active: ci % 17 !== 3, created_at: ts("2026-10-01T00:00:00.000Z", ci), updated_at: ts("2026-10-02T03:04:05.678Z", ci),
  };
  ci++;
  bySlug.set(c.slug, c);
  state.companies.set(c.id, c);
}
for (const extra of ["zz-empty-co", "aa-inactive-co", "tesla"]) {
  const c = { id: uuid(), name: extra.toUpperCase(), slug: extra, ats: extra === "tesla" ? "tesla" : "greenhouse", board_token: extra,
    careers_url: null, website_url: null, active: extra !== "aa-inactive-co", created_at: ts("2026-10-01T00:00:00.000Z", 0), updated_at: ts("2026-10-01T00:00:00.000Z", 1) };
  bySlug.set(extra, c);
  state.companies.set(c.id, c);
}
const delisted = [...DELISTED_JOBS.keys()];
dump.jobs.forEach((j: any, i: number) => {
  const c = bySlug.get(j.company.slug);
  const nt = normalizeTitle(j.title);
  const id = i < delisted.length ? delisted[i] : j.id;
  const rec = {
    id, company_id: c.id, external_job_id: `ext-${i}`, fingerprint: jobFingerprint(c.id, j.source, `ext-${i}`),
    title: i % 97 === 5 ? j.title + "  <b>&amp;</b> 🚀" : j.title, normalized_title: nt, role_family_id: roleFamilyId(c.slug, nt),
    locations: j.locations ?? [], apply_url: i % 211 === 7 ? "https://evil.example/apply" : i % 223 === 9 ? "http://" + j.apply_url.slice(8) : j.apply_url,
    excerpt: i % 50 === 1 ? "Line one,\nwith \"quotes\", commas" : j.excerpt,
    terms: j.terms, term_years: i % 61 === 2 ? [{ term: "summer", year: 2025 }] : j.term_years, duration_months: j.duration_months,
    cohort_year: j.cohort_year, roles: i % 89 === 4 ? [] : j.roles, regions: j.regions, is_remote: j.is_remote,
    is_active: i % 41 !== 6, source: j.source,
    posted_at: j.posted_at == null || i % 13 === 0 ? null : ts(j.posted_at, i),
    first_seen_at: i % 71 === 3 ? ts("2026-05-01T00:00:00.000Z", i) : ts(j.first_seen_at, i),
    created_at: ts(j.first_seen_at, i + 1), last_seen_at: ts(j.last_seen_at, i), updated_at: ts(j.last_seen_at, i + 2),
  };
  state.jobs.set(rec.id, rec);
});
const companies = [...state.companies.values()];
const base = Date.parse("2026-11-13T06:00:00.000Z");
for (let i = 0; i < 400; i++) {
  const c = companies[i % companies.length];
  const at = new Date(base - Math.floor(i / companies.length) * 86400000 - (i % 7) * 1000);
  const kind = i % 9;
  const err =
    kind === 0 ? `zero_match: 12 listed, 0 matched funnel fetched=12 title=0 tech=0 upserted=0` :
    kind === 1 ? `fetch failed: http 503 Service Unavailable` :
    kind === 2 ? `funnel fetched=${i} title=${i >> 1} tech=${i >> 2} upserted=${i >> 3} description_missing=${i % 5}` :
    kind === 3 ? `funnel fetched=00${i} title=1 tech=1 upserted=1` :
    kind === 4 ? `funnel fetched=999999999999999999 title=1 tech=1 upserted=1` : null;
  state.runs.push({
    id: uuid(), company_id: i % 53 === 10 ? (null as any) : c.id, status: kind === 1 || kind === 7 ? "error" : "ok",
    job_count: kind === 5 ? 0 : i % 30, error: err, ran_at: ts(at.toISOString(), i),
  });
}
// Same instant written two ways, one without a fraction: the string sort differs.
state.runs.push({ id: "same-a", company_id: companies[0].id, status: "ok", job_count: 1, error: null, ran_at: "2026-11-13 07:00:00+00" });
state.runs.push({ id: "same-b", company_id: companies[1].id, status: "error", job_count: 0, error: "x", ran_at: "2026-11-13 07:00:00.5+00" });
await writeState(out, state);
console.log(`${state.companies.size} companies, ${state.jobs.size} jobs, ${state.runs.length} runs`);
