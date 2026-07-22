import path from "node:path";
import { fileURLToPath } from "node:url";
import { config as loadDotenv } from "dotenv";
import type { NextConfig } from "next";

// Load monorepo root .env (apps/web is not the env root).
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
loadDotenv({ path: path.join(root, ".env") });

const nextConfig: NextConfig = {
  transpilePackages: ["@openintern/db"],
  experimental: {
    // TypeScript 7 has no JS Compiler API; run project-local `tsc` instead.
    useTypeScriptCli: true,
    serverActions: {
      bodySizeLimit: "1mb",
    },
  },
};

export default nextConfig;
