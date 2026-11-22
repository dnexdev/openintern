import assert from "node:assert/strict";
import { APPLY_HOSTS, applyUrlAllowed, checkApplyUrl } from "@openintern/db";
import { derivedHosts, parseApplyHostsTsv } from "./gen-apply-hosts.js";

const table = {
  acme: [["job-boards.greenhouse.io", "/acme/"], ["acme.wd5.myworkdayjobs.com", "/"]] as const,
};
const ok = (slug: string, url: string) => checkApplyUrl(slug, url, table).ok;
const reason = (slug: string, url: string) => {
  const c = checkApplyUrl(slug, url, table);
  return c.ok ? "ok" : c.reason;
};

assert.equal(ok("acme", "https://job-boards.greenhouse.io/acme/jobs/1"), true);
assert.equal(ok("acme", "https://job-boards.greenhouse.io/ACME/jobs/1"), true, "prefix ignores case");
assert.equal(ok("acme", "https://job-boards.greenhouse.io/acme"), true, "bare board path");
assert.equal(reason("acme", "https://job-boards.greenhouse.io/acmescam/jobs/1"), "off-list", "whole segments only");
assert.equal(reason("acme", "https://job-boards.greenhouse.io/evil/jobs/1"), "off-list", "another tenant on a shared host");
assert.equal(reason("acme", "http://job-boards.greenhouse.io/acme/jobs/1"), "not-https");
assert.equal(reason("acme", "javascript:alert(1)"), "not-https");
assert.equal(reason("acme", "https://user:pw@job-boards.greenhouse.io/acme/"), "credentials-or-port");
assert.equal(reason("acme", "https://job-boards.greenhouse.io:8443/acme/"), "credentials-or-port");
assert.equal(reason("acme", "https://acme.wd5.myworkdayjobs.com.evil.com/"), "off-list", "suffix tricks");
assert.equal(ok("acme", "https://acme.wd5.myworkdayjobs.com/Careers/job/x"), true);
assert.equal(reason("nobody", "https://job-boards.greenhouse.io/acme/"), "unknown-company");
assert.equal(reason("acme", "not a url"), "malformed");

// warn mode never hides; enforce does.
assert.equal(applyUrlAllowed("nobody", "https://evil.example/", "warn"), true);
assert.equal(applyUrlAllowed("nobody", "https://evil.example/", "enforce"), false);
assert.equal(applyUrlAllowed("nobody", "https://evil.example/", "off"), true);

assert.deepEqual(derivedHosts("lever", "Acme"), [["jobs.lever.co", "/acme/"], ["jobs.eu.lever.co", "/acme/"]]);
assert.deepEqual(derivedHosts("workday", "nvidia|wd5|NVIDIAExternalCareerSite"), [["nvidia.wd5.myworkdayjobs.com", "/"]]);
assert.deepEqual(derivedHosts("recruitee", "evil.com/x"), [], "no hosts from a malformed token");
assert.deepEqual(derivedHosts("phenom", "amd"), [], "no safe pattern: needs an explicit row");
assert.throws(() => parseApplyHostsTsv("acme\tEVIL.com\t/\n"), /bad host/);
assert.throws(() => parseApplyHostsTsv("acme\tx.com\tnoslash\n"), /prefix/);

// The compiled table accepts the live corpus it was seeded from.
assert.equal(checkApplyUrl("khan-academy", "https://job-boards.greenhouse.io/khanacademy/jobs/8250259").ok, true);
assert.equal(checkApplyUrl("lyft", "https://app.careerpuck.com/job-board/lyft/job/8797819002").ok, true);
assert.equal(checkApplyUrl("lyft", "https://app.careerpuck.com/job-board/evil/job/1").ok, false);
assert.equal(Object.keys(APPLY_HOSTS).length > 500, true);
console.log("apply-hosts ok");
