import {
  excerptFromHtml,
  extractCohortYear,
  extractDurationMonthsList,
  extractRegions,
  extractRoles,
  extractTermYears,
  extractTerms,
  isStaleByAge,
  isStaleByTermYears,
  isTechInternship,
  looksRemote,
} from "./classifier.js";

function assert(cond: boolean, msg: string) {
  if (!cond) throw new Error(msg);
}

function assertEq<T>(actual: T, expected: T, msg: string) {
  const a = JSON.stringify(actual);
  const e = JSON.stringify(expected);
  if (a !== e) throw new Error(`${msg}: expected ${e}, got ${a}`);
}

assert(isTechInternship("Software Engineer Intern"), "swe intern");
assert(isTechInternship("Machine Learning Co-op"), "ml coop");
assert(isTechInternship("Product Manager Intern"), "pm intern");
assert(isTechInternship("Technical Support Engineer Intern"), "support eng intern");
assert(isTechInternship("Engineering Internship", "Build software systems in TypeScript"), "generic eng + tech desc");
assert(isTechInternship("Full Stack Software Engineer Intern - Winter 2027"), "fullstack winter");
assert(!isTechInternship("New Grad Software Engineer"), "new grad");
assert(!isTechInternship("Marketing Intern"), "marketing");
assert(!isTechInternship("University Recruiter, Contract"), "recruiter");
assert(!isTechInternship("Industrial Engineering Internship"), "industrial eng");
assert(!isTechInternship("Intern Process Engineer"), "process eng");
assert(!isTechInternship("Mechanical Engineering Co-op"), "mechanical eng");
assert(!isTechInternship("Quality Engineer Intern"), "quality eng");
assert(!isTechInternship("Sales Project Manager Intern"), "sales pm");
assert(!isTechInternship("Engineering Internship (8h) - Tires"), "tires intern");
assert(!isTechInternship("Global Trade Compliance Intern"), "trade compliance");
assert(!isTechInternship("Fall 2026 Business Operations Internship/Co-op"), "biz ops");
assert(!isTechInternship("SDR Intern - Munich"), "sdr");
assert(!isTechInternship("Government Affairs Intern"), "gov affairs");
assert(isTechInternship("Software Engineer Intern (Fall / Winter 2026)"), "cohere-style");
assertEq(
  extractRegions(["Flexible - Any SpaceX Site"], false),
  ["us"],
  "spacex site → us",
);

assert(looksRemote(["Remote - US"], "Engineer Intern"), "remote loc");
assert(!looksRemote(["New York, NY"], "Engineer Intern"), "nyc");

// spring → summer
assertEq(extractTerms("Software Intern (Spring 2026)"), ["summer"], "spring→summer");
assertEq(extractTerms("Software Intern (Summer 2026)"), ["summer"], "summer");
assertEq(
  extractTerms("Fall or Winter co-op, autumn start"),
  ["fall", "winter"],
  "multi terms",
);
assertEq(extractTerms("Software Engineer Intern"), [], "no terms");

assertEq(extractDurationMonthsList("4 to 6 months"), [4, 5, 6], "range 4-6");
assertEq(extractDurationMonthsList("a 4-month co-op"), [4], "single 4");
assertEq(extractDurationMonthsList("16-week summer program"), [4], "16 weeks");
assertEq(extractDurationMonthsList("Software Intern"), [], "no duration");

assertEq(extractCohortYear("Software Intern (Summer 2026)"), 2026, "season year");
assertEq(extractCohortYear("Software Engineer Intern"), null, "no year");

assertEq(
  extractTermYears("Winter 2026 Software Intern", 2026),
  [{ term: "winter", year: 2026 }],
  "winter 2026 pair",
);
assertEq(
  extractTermYears("Summer Intern", 2027),
  [{ term: "summer", year: 2027 }],
  "backfill cohort onto summer",
);

assert(
  isStaleByTermYears([{ term: "winter", year: 2026 }], new Date("2026-07-09T12:00:00Z")),
  "winter 2026 stale in July",
);
assert(
  !isStaleByTermYears([{ term: "fall", year: 2026 }], new Date("2026-07-09T12:00:00Z")),
  "fall 2026 not stale in July",
);
assert(!isStaleByTermYears([], new Date("2026-07-09T12:00:00Z")), "empty not stale");

assert(
  isStaleByAge({
    termYears: [],
    postedAt: new Date("2024-09-30T00:00:00Z"),
    firstSeenAt: new Date("2026-07-01T00:00:00Z"),
    now: new Date("2026-07-09T12:00:00Z"),
  }),
  "old postedAt with empty term_years is stale",
);
assert(
  !isStaleByAge({
    termYears: [{ term: "fall", year: 2026 }],
    postedAt: new Date("2024-09-30T00:00:00Z"),
    firstSeenAt: new Date("2026-07-01T00:00:00Z"),
    now: new Date("2026-07-09T12:00:00Z"),
  }),
  "term_years present → age guard skipped",
);

assertEq(extractRoles("Software Engineer Intern"), ["software"], "swe role");
assertEq(extractRoles("ML Backend Intern"), ["backend", "ml"], "multi roles");
assertEq(extractRoles("Data Science Co-op"), ["data"], "data role");
assertEq(extractRoles("Industrial Engineering Internship"), [], "industrial no role");
assertEq(extractRoles("Engineering Internship"), [], "bare eng no software role");
assertEq(
  extractRoles("Intern", "Looking for a software developer to join our team"),
  ["software"],
  "desc software fallback",
);

assertEq(
  extractRegions(["San Francisco, CA"], false),
  ["us"],
  "sf → us",
);
assertEq(
  extractRegions(["Foster City, CA", "Remote"], true),
  ["remote", "us"],
  "foster city + remote → us+remote",
);
assertEq(
  extractRegions(["Toronto, ON", "Remote"], true),
  ["remote", "canada"],
  "toronto+remote",
);
assertEq(extractRegions(["London, UK"], false), ["europe"], "london → europe");

assertEq(
  excerptFromHtml("<p>Hello &amp; welcome</p>"),
  "Hello & welcome",
  "basic html",
);

console.log("classifier ok");
