// Regenerate with:
// packages/ingest/node_modules/.bin/tsx internal/jobslug/testdata/gen-goldens.ts
import { readFileSync, writeFileSync } from "node:fs";
import { gzipSync } from "node:zlib";
import { jobPath, jobSlugSegment, parseIdPrefixFromJobSlug, slugifyTitle } from "../../../apps/web/src/lib/job-slug.ts";

type DumpJob = { id: string; title: string; company: { slug: string } };
const root = new URL("../../../", import.meta.url);
const dump = JSON.parse(readFileSync(new URL(".cache/dump/jobs.json", root), "utf8")) as { jobs: DumpJob[] };

function fullwidthASCII(value: string): string {
  return value.replace(/[!-~]/g, (char) => String.fromCodePoint(char.codePointAt(0)! + 0xfee0));
}
function shuffled(value: string, seed: number): string {
  const tokens = value.split(/\s+/).filter(Boolean);
  for (let i = tokens.length - 1; i > 0; i--) {
    const at = ((seed * 1664525 + i * 1013904223) >>> 0) % (i + 1);
    [tokens[i], tokens[at]] = [tokens[at]!, tokens[i]!];
  }
  return tokens.join(" ");
}

const inputs = dump.jobs.map((job, i) => ({ key: `real:${i}`, ...job }));
for (let i = 0; i < Math.min(800, dump.jobs.length); i++) {
  const job = dump.jobs[i]!;
  const titles = [
    job.title.toUpperCase(),
    fullwidthASCII(job.title),
    `🧑‍💻 ${job.title} !!! — 🚀`,
    shuffled(job.title, i + 1),
    `${job.title} — Summer 2027 / 4 to 6 months`,
    `２０２７：${job.title.replace(/[-–—,/()]/g, " • ")}（Autumn）`,
  ];
  titles.forEach((title, variant) => inputs.push({ key: `synthetic:${i}:${variant}`, id: job.id, title, company: job.company }));
}

const cases = inputs.map((input) => {
  const segment = jobSlugSegment(input.title, input.id);
  return {
    input: { key: input.key, id: input.id, title: input.title, companySlug: input.company.slug },
    output: {
      slugified: slugifyTitle(input.title),
      segment,
      path: jobPath(input.company.slug, input.title, input.id),
      parsed: parseIdPrefixFromJobSlug(segment),
    },
  };
});
const malformed = ["", "12345678", "a-1234567", "a-1234567g", "a-ABCDEF12", "a--deadbeef", "日本語-0123abcd", "😀-deadbeef"].map((slug) => ({ slug, parsed: parseIdPrefixFromJobSlug(slug) }));
const payload = JSON.stringify({ realCount: dump.jobs.length, syntheticCount: inputs.length - dump.jobs.length, cases, malformed });
writeFileSync(new URL("./goldens.json.gz", import.meta.url), gzipSync(payload, { level: 9 }));
console.log(`wrote ${cases.length} job slug cases (${dump.jobs.length} real, ${inputs.length - dump.jobs.length} synthetic)`);
