import path from "node:path";
import { fileURLToPath } from "node:url";
import { config } from "dotenv";

/** Load repo-root `.env` then cwd `.env` (later wins for overrides). */
export function loadEnv() {
  const here = path.dirname(fileURLToPath(import.meta.url));
  // packages/db/src -> repo root is ../../..
  // packages/ingest/src -> same
  const root = path.resolve(here, "../../..");
  config({ path: path.join(root, ".env") });
  config({ path: path.join(process.cwd(), ".env"), override: true });
}
