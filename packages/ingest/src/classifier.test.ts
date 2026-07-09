import {
  excerptFromHtml,
  extractCohortYear,
  extractDurationMonths,
  extractTerms,
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

// --- isTechInternship: internships only, tech only ---

assert(isTechInternship("Software Engineer Intern"), "swe intern");
assert(isTechInternship("Software Engineering Intern"), "swe-ing intern");
assert(isTechInternship("Machine Learning Co-op"), "ml coop");
assert(isTechInternship("Data Science Co-op"), "data coop");
assert(isTechInternship("Fall 2026 Security Internship"), "security internship");
// Generic title, tech description
assert(
  isTechInternship("Intern", "You will write software with our backend engineering team"),
  "generic intern w/ tech description",
);

// Full-time roles must not pass, even with intern-ish words in description
assert(!isTechInternship("New Grad Software Engineer"), "new grad is not an internship");
assert(
  !isTechInternship("University Recruiter, Contract", "Recruit interns from campus"),
  "university recruiter",
);
assert(!isTechInternship("Senior Software Engineer"), "senior");
assert(!isTechInternship("Staff Product Manager"), "staff");
assert(
  !isTechInternship("Software Engineer", "Work with our interns and university hires"),
  "full-time role mentioning interns in description",
);

// Non-tech internships must not pass
assert(!isTechInternship("Marketing Intern"), "marketing intern");
assert(
  !isTechInternship("Marketing Intern", "Work alongside our engineering team"),
  "marketing intern w/ tech boilerplate description",
);
assert(!isTechInternship("HR Intern"), "hr intern");
assert(!isTechInternship("Talent Acquisition Intern"), "talent intern");
assert(!isTechInternship("Apprentice Electrician"), "electrician apprentice");
assert(
  !isTechInternship("Intern", "Support our retail store operations"),
  "generic intern, non-tech description",
);

// --- looksRemote ---

assert(looksRemote(["Remote - US"], "Engineer Intern"), "remote loc");
assert(!looksRemote(["New York, NY"], "Engineer Intern"), "nyc");

// --- extractTerms ---

assertEq(extractTerms("Software Intern (Summer 2026)"), ["summer"], "summer term");
assertEq(
  extractTerms("Fall or Winter co-op, autumn start possible"),
  ["winter", "fall"],
  "multi terms normalized/ordered",
);
assertEq(extractTerms("Software Engineer Intern"), [], "no terms");

// --- extractDurationMonths ---

assertEq(extractDurationMonths("This is a 4-month co-op placement"), 4, "4 month");
assertEq(extractDurationMonths("a 12 month internship in London"), 12, "12 month");
assertEq(extractDurationMonths("our 16-week summer program"), 4, "16 weeks -> 4 months");
assertEq(extractDurationMonths("Software Intern"), null, "no duration");

// --- extractCohortYear ---

assertEq(extractCohortYear("Software Intern (Summer 2026)"), 2026, "season year");
assertEq(extractCohortYear("Class of 2027 Software Engineer Intern"), 2027, "class of");
assertEq(extractCohortYear("2027 - Software Engineering Intern"), 2027, "year before intern");
assertEq(extractCohortYear("Software Engineer Intern"), null, "no year");
assertEq(extractCohortYear("Intern in 1999"), null, "year out of window");

// --- excerptFromHtml ---

assertEq(
  excerptFromHtml("<p>Hello &amp; welcome</p>"),
  "Hello & welcome",
  "basic html excerpt",
);
assertEq(
  excerptFromHtml("<div class=&quot;p-pdf&quot;> <div class=&quot;textLayer&quot;>Real content here</div></div>"),
  "Real content here",
  "double-encoded html stripped",
);

console.log("classifier ok");
