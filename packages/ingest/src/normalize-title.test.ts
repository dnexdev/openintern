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

// Campus program variants → one family (IMC/Jump India recruiting)
const campusBase = normalizeTitle("Software Engineer Intern - IIT Madras");
assert.equal(normalizeTitle("Software Engineer Intern - IIT Bombay"), campusBase);
assert.equal(normalizeTitle("Software Engineer Intern - BITS Pilani"), campusBase);
assert.equal(normalizeTitle("Software Engineer Intern, Campus"), campusBase);
assert.equal(
  normalizeTitle("2027 - Software Engineering Intern - IIT Delhi"),
  normalizeTitle("Software Engineering Intern - IIT Bombay"),
);

// Degree tracks stay separate families (eligibility differs)
assert.notEqual(
  normalizeTitle("Campus Quantitative Researcher, UG/MS (Intern)"),
  normalizeTitle("Campus Quantitative Researcher, PhD (Intern)"),
);
assert.notEqual(
  normalizeTitle("Quantitative Research Intern (PhD) - Summer 2027"),
  normalizeTitle("Quantitative Research Intern (BS/MS) - Summer 2027"),
);
assert.notEqual(
  normalizeTitle("Research Intern (BS/MS/PhD)"),
  normalizeTitle("Research Intern"),
);

// Specialty/domain suffixes stay separate (different roles)
assert.notEqual(
  normalizeTitle("Campus AI Research Engineer (Intern)"),
  normalizeTitle("Campus AI Research Engineer - Deep Learning (Intern)"),
);
assert.notEqual(
  normalizeTitle("Campus AI Research Engineer (Intern)"),
  normalizeTitle("Campus AI Research Engineer – Research Automation (Intern)"),
);

// Location + season noise still collapses within the same degree/specialty track
assert.equal(
  normalizeTitle("Quantitative Research Intern (PhD) - Summer 2027 - Chicago"),
  normalizeTitle("Quantitative Research Intern (PhD) - Summer 2027 - Amsterdam"),
);
assert.equal(
  normalizeTitle("Machine Learning Research Intern - Summer 2027 - Chicago"),
  normalizeTitle("Machine Learning Research Intern - Summer 2027 - Amsterdam"),
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
