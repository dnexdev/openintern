import fs from "node:fs/promises";
import path from "node:path";
import { createDbClient } from "@openintern/db";
import { backfillRoleFamilies } from "./backfill-role-families.js";
import { writeDumps } from "./dump.js";
import { writeDumpsFromState } from "./dump-state.js";
import { importState } from "./import-state.js";
import { runIngestFiles } from "./ingest-files.js";
import { loadState, stateFromExport, writeState } from "./state.js";
import { ingestDigest, ingestStepSummary, runIngest } from "./ingest.js";
import { loadEnv } from "./load-env.js";
import { syncCompaniesFromYaml } from "./sync-companies.js";

loadEnv();

async function main() {
  const cmd = process.argv[2] ?? "ingest";

  // Flat-file state commands (plan phase D) need no database.
  if (cmd === "state-from-export") {
    const [exportDir, stateDir] = process.argv.slice(3);
    if (!exportDir || !stateDir) throw new Error("usage: state-from-export <export-dir> <state-dir>");
    const state = await stateFromExport(exportDir);
    await writeState(stateDir, state);
    console.log(`state: ${state.companies.size} companies, ${state.jobs.size} jobs, ${state.runs.length} runs → ${stateDir}`);
    return;
  }
  if (cmd === "ingest-state") {
    const stateDir = process.argv[3];
    if (!stateDir) throw new Error("usage: ingest-state <state-dir>");
    const state = await loadState(stateDir);
    const only = process.env.OPENINTERN_ONLY_SLUGS?.trim();
    const summary = await runIngestFiles(state, {
      only: only ? new Set(only.split(",").map((x) => x.trim()).filter(Boolean)) : undefined,
    });
    await writeState(stateDir, state);
    console.log(JSON.stringify(summary, null, 2));
    const digestPath = process.env.OPENINTERN_DIGEST_PATH?.trim();
    const digest = ingestDigest(summary);
    if (digestPath && digest) await fs.writeFile(digestPath, digest);
    const stepSummary = process.env.GITHUB_STEP_SUMMARY?.trim();
    if (stepSummary) await fs.appendFile(stepSummary, ingestStepSummary(summary));
    if (summary.companies > 0 && summary.jobsUpserted === 0 && summary.failures.length > 0) process.exitCode = 1;
    return;
  }
  if (cmd === "dump-state") {
    const [stateDir, outDir] = process.argv.slice(3);
    if (!stateDir || !outDir) throw new Error("usage: dump-state <state-dir> <out-dir>");
    const result = await writeDumpsFromState(await loadState(stateDir), outDir);
    console.log(`Wrote ${result.count} jobs to ${outDir}`);
    return;
  }

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
        `Wrote ${result.count} jobs to ${result.jsonPath}, ${result.csvPath} and ${result.healthPath}`,
      );
      return;
    }

    if (cmd === "import-state") {
      const stateDir = process.argv[3];
      if (!stateDir) throw new Error("usage: import-state <state-dir>");
      console.log(JSON.stringify(await importState(db, await loadState(stateDir))));
      return;
    }

    if (cmd === "backfill-role-families") {
      const result = await backfillRoleFamilies(db);
      console.log(JSON.stringify(result, null, 2));
      return;
    }

    if (cmd === "ingest") {
      const summary = await runIngest(db);
      console.log(JSON.stringify(summary, null, 2));
      // CI hands the digest to a separate job that may open issues.
      const digestPath = process.env.OPENINTERN_DIGEST_PATH?.trim();
      const digest = ingestDigest(summary);
      if (digestPath && digest) await fs.writeFile(digestPath, digest);
      const stepSummary = process.env.GITHUB_STEP_SUMMARY?.trim();
      if (stepSummary) await fs.appendFile(stepSummary, ingestStepSummary(summary));
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
    console.error(
      "Usage: cli.ts <ingest|sync-companies|dump|backfill-role-families> [dump-dir]",
    );
    process.exitCode = 1;
  } finally {
    await client.end({ timeout: 5 });
  }
}

main().catch(async (err) => {
  console.error(err);
  process.exit(1);
});
