import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { validateCompanies } from "./validate-companies.js";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

const dir = await fs.mkdtemp(path.join(os.tmpdir(), "oi-companies-"));
await fs.writeFile(
  path.join(dir, "a.yaml"),
  `companies:
  - name: Alpha
    slug: alpha
    ats: greenhouse
    board_token: shared
`,
);
await fs.writeFile(
  path.join(dir, "b.yaml"),
  `companies:
  - name: Alpha duplicate
    slug: alpha
    ats: lever
    board_token: alpha
  - name: Beta
    slug: beta
    ats: greenhouse
    board_token: shared
    website_url: https://beta.example
`,
);

const result = await validateCompanies(dir);
assert(result.errors.some((e) => e.includes('duplicate slug "alpha"')), "duplicate slug");
assert(
  result.warnings.some((w) => w.includes('duplicate ATS token "greenhouse:shared"')),
  "duplicate ATS token warning",
);
assert(
  result.warnings.some((w) => w.includes("missing website_url for alpha")),
  "missing website warning",
);

console.log("validate-companies ok");
