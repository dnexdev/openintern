import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
const src = process.argv[2]!; // packages/ingest/src absolute
const td = process.argv[3]!;  // internal/ats/testdata/manual absolute
const out = process.argv[4]!;
const { CITADEL_ORIGINS, citadelListingUrl, parseCitadelListingHtml } = await import(path.join(src, "citadel.ts"));
const { TESLA_STATE_URL, parseTeslaCareersState } = await import(path.join(src, "tesla.ts"));
const { toManualCapture, loadManualJobs } = await import(path.join(src, "manual.ts"));
const now = new Date("2026-11-20T08:00:00Z");
const seen = new Set<string>();
const citadel: any[] = [];
for (const f of ["citadel-page1.html", "citadel-page2.html"]) {
  for (const j of parseCitadelListingHtml(await fs.readFile(path.join(td, f), "utf8"), CITADEL_ORIGINS.citadel)) {
    if (seen.has(j.externalId)) continue; seen.add(j.externalId); citadel.push(j);
  }
}
const tesla = parseTeslaCareersState(JSON.parse(await fs.readFile(path.join(td, "tesla-state.json"), "utf8")));
await fs.writeFile(path.join(out, "parsed.golden.json"), JSON.stringify({ citadel, tesla }, null, 1) + "\n");
const dir = await fs.mkdtemp(path.join(os.tmpdir(), "oi-manual-"));
for (const [name, jobs, source] of [["citadel", citadel, citadelListingUrl("citadel", "open-opportunities")], ["tesla", tesla, TESLA_STATE_URL]] as const) {
  const cap = toManualCapture(jobs, { source, now });
  const text = JSON.stringify(cap, null, 2) + "\n";
  await fs.writeFile(path.join(out, `${name}.capture.golden.json`), text);
  await fs.writeFile(path.join(dir, `${name}.json`), text);
  const loaded = await loadManualJobs(name, { dir, now: new Date(now.getTime() + 86400000) });
  await fs.writeFile(path.join(out, `${name}.jobs.golden.json`), JSON.stringify(loaded, null, 1) + "\n");
}
