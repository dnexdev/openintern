import assert from "node:assert/strict";
import { extractCompaniesFromMarkdown } from "./gap-report.js";
import {
  extractAtsBoardsFromMarkdown,
  parseAtsFromApplyUrl,
  tokenKey,
} from "./parse-ats-url.js";

const md = `
| Company | Position | Age |
|---|---|---|
| Palantir | Software Engineer Intern | 8d |
| **Jane Street** | SWE Intern | 3d |
| [Notion](https://notion.so) | Software Engineer Intern | 1d |

<table>
<tr><td>Company</td><td>Role</td></tr>
<tr><td>Anduril</td><td>2027 Software Engineer Intern</td></tr>
<tr><td><a href="https://www.spacex.com"><strong>SpaceX</strong></a></td><td>Fall 2026 Software Engineering Internship</td></tr>
<tr><td>🔥 Meta</td><td>Software Engineer Intern</td></tr>
</table>
`;

const names = extractCompaniesFromMarkdown(md);
assert.ok(names.includes("Palantir"), "palantir");
assert.ok(names.includes("Jane Street"), "jane street");
assert.ok(names.includes("Notion"), "notion");
assert.ok(names.includes("Anduril"), "anduril");
assert.ok(names.includes("SpaceX"), "spacex");
assert.ok(names.includes("Meta"), "meta without emoji");
assert.ok(!names.includes("Company"), "skip header");
assert.ok(!names.some((n) => n.includes("<")), "no raw html");
assert.ok(!names.some((n) => n.includes("🔥")), "no emoji prefix");

// ATS URL parsing
assert.deepEqual(parseAtsFromApplyUrl("https://jobs.ashbyhq.com/bloxd/7ade559a/application"), {
  ats: "ashby",
  boardToken: "bloxd",
  careersUrl: "https://jobs.ashbyhq.com/bloxd",
});
assert.deepEqual(
  parseAtsFromApplyUrl("https://job-boards.greenhouse.io/later/jobs/8621762002"),
  {
    ats: "greenhouse",
    boardToken: "later",
    careersUrl: "https://job-boards.greenhouse.io/later",
  },
);
assert.equal(
  parseAtsFromApplyUrl(
    "https://boards.greenhouse.io/embed/job_app?token=7975026&utm_source=Simplify",
  ),
  null,
  "greenhouse embed job id is not a board token",
);
assert.deepEqual(
  parseAtsFromApplyUrl(
    "https://selinc.wd1.myworkdayjobs.com/SEL/job/Idaho---Boise/Software-Engineer-Intern",
  ),
  {
    ats: "workday",
    boardToken: "selinc|wd1|SEL",
    careersUrl: "https://selinc.wd1.myworkdayjobs.com/SEL",
  },
  "workday CXS board from apply URL",
);
assert.equal(tokenKey("greenhouse", "Later"), "greenhouse:later");
assert.equal(tokenKey("ashby", "Bloxd"), "ashby:bloxd");
assert.equal(
  tokenKey("workday", "selinc|wd1|SEL"),
  "workday:selinc|wd1|SEL",
);

const table = `
| Company | Role | Loc | Application | Age |
|---|---|---|---|---|
| **[Bloxd](https://simplify.jobs/c/x)** | SWE Intern | London | [Apply](https://jobs.ashbyhq.com/bloxd/abc/application) [Simplify](https://simplify.jobs/p/1) | 1d |
| **[Later](https://simplify.jobs/c/Later)** | Co-op | Vancouver | [Apply](https://job-boards.greenhouse.io/later/jobs/1) [Simplify](https://simplify.jobs/p/2) | 2d |
| **[Acme](https://simplify.jobs/c/Acme)** | Intern | SF | [Apply](https://acme.wd5.myworkdayjobs.com/Careers/job/x) [Simplify](https://simplify.jobs/p/3) | 3d |
| ↳ | Another Intern | SF | [Apply](https://jobs.ashbyhq.com/bloxd/def) [Simplify](https://simplify.jobs/p/4) | 3d |
`;

const extracted = extractAtsBoardsFromMarkdown(table);
assert.equal(extracted.applyLinkCount, 4);
assert.equal(extracted.unsupportedApplyCount, 0);
assert.equal(extracted.hits.length, 4);
assert.ok(extracted.hits.some((h) => h.companyName === "Bloxd" && h.boardToken === "bloxd"));
assert.ok(extracted.hits.some((h) => h.companyName === "Later" && h.ats === "greenhouse"));
assert.ok(extracted.hits.some((h) => h.ats === "workday" && h.boardToken === "acme|wd5|Careers"));
// Continuation row inherits Acme but apply is bloxd — company name from lastCompany after Acme
assert.ok(extracted.hits.some((h) => h.boardToken === "bloxd"));

// HTML table format (live SimplifyJobs README)
const htmlTable = `
<table>
<thead><tr><th>Company</th><th>Role</th><th>Application</th></tr></thead>
<tbody>
<tr>
<td><strong><a href="https://simplify.jobs/c/Bloxd">Bloxd</a></strong></td>
<td>Software Engineer Intern</td>
<td><div align="center"><a href="https://jobs.ashbyhq.com/bloxd/7ade559a/application?embed=true"><img src="x.png" width="50" alt="Apply"></a></div></td>
</tr>
<tr>
<td><strong><a href="https://simplify.jobs/c/Later">Later</a></strong></td>
<td>Co-op</td>
<td><div align="center"><a href="https://job-boards.greenhouse.io/later/jobs/1"><img src="x.png" width="50" alt="Apply"></a></div></td>
</tr>
<tr>
<td><strong><a href="https://simplify.jobs/c/Boeing">Boeing</a></strong></td>
<td>Intern</td>
<td><div align="center"><a href="https://boeing.wd1.myworkdayjobs.com/CAREERS"><img src="x.png" width="50" alt="Apply"></a></div></td>
</tr>
</tbody>
</table>
`;
const htmlEx = extractAtsBoardsFromMarkdown(htmlTable);
assert.equal(htmlEx.applyLinkCount, 3);
assert.equal(htmlEx.unsupportedApplyCount, 0);
assert.equal(htmlEx.hits.length, 3);
assert.ok(htmlEx.hits.some((h) => h.companyName === "Bloxd" && h.ats === "ashby"));
assert.ok(htmlEx.hits.some((h) => h.companyName === "Later" && h.boardToken === "later"));
assert.ok(
  htmlEx.hits.some(
    (h) => h.companyName === "Boeing" && h.ats === "workday" && h.boardToken === "boeing|wd1|CAREERS",
  ),
);

console.log("gap-report ok");
