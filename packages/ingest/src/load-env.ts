import path from "node:path";
import { fileURLToPath } from "node:url";
import { config } from "dotenv";

export function loadEnv() {
  const here = path.dirname(fileURLToPath(import.meta.url));
  const root = path.resolve(here, "../../..");
  config({ path: path.join(root, ".env") });
  config({ path: path.join(process.cwd(), ".env"), override: true });
}
