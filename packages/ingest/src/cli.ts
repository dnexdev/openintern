import path from "node:path";
import { createDbClient } from "@openintern/db";
import { collectAlertMatches } from "./alerts.js";
import { writeDumps } from "./dump.js";
import { runIngest } from "./ingest.js";
import { loadEnv } from "./load-env.js";
import { syncCompaniesFromYaml } from "./sync-companies.js";

loadEnv();

async function deliverAlerts(
  matches: Awaited<ReturnType<typeof collectAlertMatches>>,
) {
  const apiKey = process.env.RESEND_API_KEY;
  const from = process.env.ALERT_FROM_EMAIL ?? "alerts@openintern.dev";

  for (const m of matches) {
    if (m.webhookUrl) {
      try {
        await fetch(m.webhookUrl, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            search: m.searchName,
            count: m.jobs.length,
            jobs: m.jobs,
          }),
        });
        console.log(`Webhook delivered for search ${m.searchName}`);
      } catch (err) {
        console.error(`Webhook failed for ${m.searchName}:`, err);
      }
    }

    if (m.emailEnabled && apiKey) {
      const lines = m.jobs
        .map((j) => `- ${j.title} @ ${j.company}: ${j.applyUrl}`)
        .join("\n");
      const res = await fetch("https://api.resend.com/emails", {
        method: "POST",
        headers: {
          Authorization: `Bearer ${apiKey}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          from,
          to: [m.userEmail],
          subject: `OpenIntern: ${m.jobs.length} new match(es) for “${m.searchName}”`,
          text: `New internship matches for your saved search “${m.searchName}”:\n\n${lines}\n\n— OpenIntern`,
        }),
      });
      if (!res.ok) {
        console.error(`Resend failed: ${res.status} ${await res.text()}`);
      } else {
        console.log(`Email sent to ${m.userEmail}`);
      }
    } else if (m.emailEnabled && !apiKey) {
      console.log(
        `[dry-run email] to=${m.userEmail} search=${m.searchName} jobs=${m.jobs.length}`,
      );
    }
  }
}

async function main() {
  const cmd = process.argv[2] ?? "ingest";
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

    if (cmd === "alerts") {
      const matches = await collectAlertMatches(db);
      console.log(`Found ${matches.length} searches with new matches.`);
      await deliverAlerts(matches);
      return;
    }

    if (cmd === "ingest") {
      const summary = await runIngest(db);
      console.log(JSON.stringify(summary, null, 2));
      if (summary.failures.length) {
        process.exitCode = 1;
      }
      return;
    }

    console.error(`Unknown command: ${cmd}`);
    console.error("Usage: cli.ts <ingest|sync-companies|dump|alerts> [dump-dir]");
    process.exitCode = 1;
  } finally {
    await client.end({ timeout: 5 });
  }
}

main().catch(async (err) => {
  console.error(err);
  process.exit(1);
});
