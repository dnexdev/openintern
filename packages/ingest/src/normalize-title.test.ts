import assert from "node:assert/strict";
import {
  jobFingerprint,
  normalizeTitle,
  roleFamilyId,
} from "./normalize-title.js";

// Palantir FDSE location/program variants → one family
const palantirBase = normalizeTitle(
  "Forward Deployed Software Engineer, Internship - Commercial",
);
assert.equal(
  normalizeTitle("Forward Deployed Software Engineer, Internship - USG"),
  palantirBase,
);
assert.equal(
  normalizeTitle("Forward Deployed Software Engineer, Internship - AUS Government"),
  palantirBase,
);
assert.equal(
  normalizeTitle("Forward Deployed Software Engineer, Internship (Fall 2026)"),
  palantirBase,
);

// Season parentheticals
assert.equal(
  normalizeTitle("Software Engineering Intern (Summer 2027)"),
  normalizeTitle("Software Engineering Intern"),
);

// Workato-style city suffix
assert.equal(
  normalizeTitle("Software Engineer Intern - San Francisco, CA"),
  normalizeTitle("Software Engineer Intern - New York, NY"),
);

// Campus program variants → one family (Jump/DMC-style India recruiting)
const campusBase = normalizeTitle("Software Engineer Intern - IIT Madras");
assert.equal(normalizeTitle("Software Engineer Intern - IIT Bombay"), campusBase);
assert.equal(normalizeTitle("Software Engineer Intern - BITS Pilani"), campusBase);
assert.equal(normalizeTitle("Software Engineer Intern, Campus"), campusBase);

// Degree parenthetical
assert.equal(
  normalizeTitle("Research Intern (BS/MS/PhD)"),
  normalizeTitle("Research Intern"),
);

// Fingerprint stability
const fp = jobFingerprint("co-1", "greenhouse", "123");
assert.equal(fp.length, 64);
assert.equal(jobFingerprint("co-1", "greenhouse", "123"), fp);
assert.notEqual(jobFingerprint("co-1", "greenhouse", "124"), fp);

// Role family id
assert.match(roleFamilyId("palantir", palantirBase), /^palantir:[a-f0-9]{10}$/);
assert.equal(
  roleFamilyId("palantir", palantirBase),
  roleFamilyId("palantir", normalizeTitle("Forward Deployed Software Engineer, Internship - USG")),
);

console.log("normalize-title.test.ts: ok");
