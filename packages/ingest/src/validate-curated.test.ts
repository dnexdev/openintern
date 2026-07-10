import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { validateTier1Curated } from "./validate-curated.js";

function assert(cond: boolean, msg: string) {
  if (!cond) throw new Error(msg);
}

const root = await fs.mkdtemp(path.join(os.tmpdir(), "oi-curated-"));
const companiesDir = path.join(root, "companies");
const curatedDir = path.join(root, "curated");
await fs.mkdir(companiesDir);
await fs.mkdir(curatedDir);

await fs.writeFile(
  path.join(companiesDir, "a.yaml"),
  `companies:
  - name: OpenAI
    slug: openai
    ats: greenhouse
    board_token: openai
`,
);
await fs.writeFile(
  path.join(curatedDir, "tier-1.yaml"),
  `slugs:
  - openai
`,
);

const ok = await validateTier1Curated({
  tier1Path: path.join(curatedDir, "tier-1.yaml"),
  companiesDir,
});
assert(ok.errors.length === 0, "valid slug passes");

await fs.writeFile(
  path.join(curatedDir, "tier-1.yaml"),
  `slugs:
  - not-a-real-slug
`,
);
const bad = await validateTier1Curated({
  tier1Path: path.join(curatedDir, "tier-1.yaml"),
  companiesDir,
});
assert(bad.errors.some((e) => e.includes("not-a-real-slug")), "unknown slug fails");

console.log("validate-curated ok");
