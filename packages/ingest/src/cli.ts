import path from "node:path";
import { createDbClient } from "@openintern/db";
import { writeDumps } from "./dump.js";
import { runIngest } from "./ingest.js";
import { loadEnv } from "./load-env.js";
import { syncCompaniesFromYaml } from "./sync-companies.js";

loadEnv();

async function main() {
  const cmd = process.argv[2] ?? "ingest";

  if (!process.env.DATABASE_URL?.trim()) {
    console.error(
      "DATABASE_URL is required. Set it in .env locally, or as a GitHub Actions secret for Ingest/Dump.",
    );
    process.exit(1);
  }

  const { db, client } = createDbClient();

  try {
    if (cmd === "sync-companies") {
      const result = await syncCompaniesFromYaml(db);
      console.log(`Synced ${result.upserted} companies from YAML.`);
      return;
    }

    if (cmd === "dump") {
      const outDir = process.argv[3] ?? path.resolve(process.cwd(), "dumps");
      const result = await writeDumps(db, outDir);
      console.log(
        `Wrote ${result.count} jobs to ${result.jsonPath} and ${result.csvPath}`,
      );
      return;
    }

    if (cmd === "ingest") {
      const summary = await runIngest(db);
      console.log(JSON.stringify(summary, null, 2));
      // Per-company ATS failures are expected (dead tokens). Only fail the
      // process when nothing succeeded — so scheduled CI stays green while
      // /health still surfaces individual errors.
      if (
        summary.companies > 0 &&
        summary.jobsUpserted === 0 &&
        summary.failures.length > 0
      ) {
        process.exitCode = 1;
      } else if (summary.failures.length) {
        console.warn(
          `Ingest completed with ${summary.failures.length} company failure(s); see /health.`,
        );
      }
      return;
    }

    console.error(`Unknown command: ${cmd}`);
    console.error("Usage: cli.ts <ingest|sync-companies|dump> [dump-dir]");
    process.exitCode = 1;
  } finally {
    await client.end({ timeout: 5 });
  }
}

main().catch(async (err) => {
  console.error(err);
  process.exit(1);
});
