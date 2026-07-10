import { extractCompaniesFromMarkdown } from "./gap-report.js";

function assert(cond: boolean, msg: string) {
  if (!cond) throw new Error(msg);
}

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
assert(names.includes("Palantir"), "palantir");
assert(names.includes("Jane Street"), "jane street");
assert(names.includes("Notion"), "notion");
assert(names.includes("Anduril"), "anduril");
assert(names.includes("SpaceX"), "spacex");
assert(names.includes("Meta"), "meta without emoji");
assert(!names.includes("Company"), "skip header");
assert(!names.some((n) => n.includes("<")), "no raw html");
assert(!names.some((n) => n.includes("🔥")), "no emoji prefix");

console.log("gap-report ok");
