// Regenerate with:
// packages/ingest/node_modules/.bin/tsx internal/classify/testdata/gen-goldens.ts
import { readFileSync, writeFileSync } from "node:fs";
import { gzipSync } from "node:zlib";
import {
  excerptFromHtml,
  extractCohortYear,
  extractDurationMonths,
  extractDurationMonthsList,
  extractRegions,
  extractRoles,
  extractTermYears,
  extractTerms,
  isStaleByAge,
  isStaleByLastSeen,
  isStaleByTermYears,
  isTechInternship,
  looksRemote,
} from "../../../packages/ingest/src/classifier.ts";
import { jobFingerprint, normalizeTitle, roleFamilyId, shortHash } from "../../../packages/ingest/src/normalize-title.ts";

type DumpJob = {
  id: string;
  title: string;
  company: { slug: string };
  locations: string[];
  excerpt: string | null;
  source: string;
  posted_at: string | null;
  first_seen_at: string;
  last_seen_at: string;
};

type Input = {
  key: string;
  title: string;
  description: string;
  locations: string[];
  id: string;
  companySlug: string;
  source: string;
  postedAt: string | null;
  firstSeenAt: string;
  lastSeenAt: string;
};

const root = new URL("../../../", import.meta.url);
const dump = JSON.parse(readFileSync(new URL(".cache/dump/jobs.json", root), "utf8")) as { jobs: DumpJob[] };
const now = new Date("2026-11-13T12:00:00.000Z");
const phraseCorpus = dump.jobs.flatMap((job) => `${job.title} ${job.excerpt ?? ""}`.match(/\b(?:winter|spring|summer|fall|autumn)\s+20\d{2}\b|\b\d{1,2}\s*(?:to|-|–|—)\s*\d{1,2}\s*(?:months?|weeks?)\b|\b\d{1,2}[-–— ](?:months?|weeks?)\b/gi) ?? []);
const phrases = [...new Set(phraseCorpus)];
if (phrases.length === 0) phrases.push("Summer 2027", "4 to 6 months", "16-week");

function fullwidthASCII(value: string): string {
  return value.replace(/[!-~]/g, (char) => String.fromCodePoint(char.codePointAt(0)! + 0xfee0));
}

function shuffled(value: string, seed: number): string {
  const tokens = value.split(/\s+/).filter(Boolean);
  for (let i = tokens.length - 1; i > 0; i--) {
    const j = (seed * 1103515245 + i * 12345) >>> 0;
    const at = j % (i + 1);
    [tokens[i], tokens[at]] = [tokens[at]!, tokens[i]!];
  }
  return tokens.join(" ");
}

function inputFrom(job: DumpJob, key: string, title = job.title, description = job.excerpt ?? ""): Input {
  return {
    key,
    title,
    description,
    locations: job.locations,
    id: job.id,
    companySlug: job.company.slug,
    source: job.source,
    postedAt: job.posted_at,
    firstSeenAt: job.first_seen_at,
    lastSeenAt: job.last_seen_at,
  };
}

const inputs: Input[] = dump.jobs.map((job, i) => inputFrom(job, `real:${i}`));
for (let i = 0; i < Math.min(800, dump.jobs.length); i++) {
  const job = dump.jobs[i]!;
  const phrase = phrases[i % phrases.length]!;
  const variants = [
    job.title.toUpperCase(),
    fullwidthASCII(job.title),
    `🧑‍💻 ${job.title} !!! — 🚀`,
    shuffled(job.title, i + 1),
    `${job.title} — ${phrase}`,
    `２０２７ — ${job.title.replace(/[-–—,/()]/g, " • ")} (Autumn 2027) 12-week`,
  ];
  variants.forEach((title, variant) => inputs.push(inputFrom(job, `synthetic:${i}:${variant}`, title, variant % 2 === 0 ? job.excerpt ?? "" : `Build software, ML, data, and hardware systems. ${phrase}`)));
}

const cases = inputs.map((input) => {
  const text = `${input.title} ${input.description}`;
  const cohortYear = extractCohortYear(text, now);
  const termYears = extractTermYears(text, cohortYear, now);
  const remote = looksRemote(input.locations, input.title);
  const normalized = normalizeTitle(input.title);
  return {
    input,
    output: {
      tech: isTechInternship(input.title, input.description),
      remote,
      terms: extractTerms(text),
      cohortYear,
      termYears,
      durationMonths: extractDurationMonthsList(text),
      firstDurationMonth: extractDurationMonths(text),
      roles: extractRoles(input.title, input.description),
      regions: extractRegions(input.locations, remote),
      excerpt: excerptFromHtml(input.description),
      normalized,
      shortHash: shortHash(normalized),
      fingerprint: jobFingerprint(input.companySlug, input.source, input.id),
      roleFamilyId: roleFamilyId(input.companySlug, normalized),
      staleByTermYears: isStaleByTermYears(termYears, now),
      staleByAge: isStaleByAge({
        termYears,
        postedAt: input.postedAt ? new Date(input.postedAt) : null,
        firstSeenAt: new Date(input.firstSeenAt),
        now,
      }),
      staleByLastSeen: isStaleByLastSeen(new Date(input.lastSeenAt), now),
    },
  };
});

const payload = JSON.stringify({ now: now.toISOString(), realCount: dump.jobs.length, syntheticCount: inputs.length - dump.jobs.length, cases });
writeFileSync(new URL("./goldens.json.gz", import.meta.url), gzipSync(payload, { level: 9 }));
console.log(`wrote ${cases.length} classifier cases (${dump.jobs.length} real, ${inputs.length - dump.jobs.length} synthetic)`);
