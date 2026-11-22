/**
 * Turn a page saved from a normal browser into a committable manual capture.
 *
 *   pnpm manual:capture tesla ~/Downloads/state.json
 *   pnpm manual:capture citadel ~/Downloads/page1.html ~/Downloads/page2.html
 *
 * Writes data/manual/<ats>.json with normalized fields only. The raw files are
 * never committed.
 */
import fs from "node:fs/promises";
import type { NormalizedJob } from "./ats.js";
import { CITADEL_ORIGINS, citadelListingUrl, parseCitadelListingHtml, type CitadelBrand } from "./citadel.js";
import { manualPath, toManualCapture } from "./manual.js";
import { TESLA_STATE_URL, parseTeslaCareersState } from "./tesla.js";

async function main() {
  const [ats, ...files] = process.argv.slice(2);
  if (!ats || files.length === 0) {
    console.error("usage: manual:capture <tesla|citadel|citadel_securities> <saved file>...");
    process.exit(2);
  }

  let jobs: NormalizedJob[];
  let source: string;
  if (ats === "tesla") {
    jobs = files.length ? parseTeslaCareersState(JSON.parse(await fs.readFile(files[0]!, "utf8"))) : [];
    source = TESLA_STATE_URL;
  } else if (ats === "citadel" || ats === "citadel_securities") {
    const brand: CitadelBrand = ats;
    const seen = new Set<string>();
    jobs = [];
    for (const file of files) {
      for (const job of parseCitadelListingHtml(await fs.readFile(file, "utf8"), CITADEL_ORIGINS[brand])) {
        if (seen.has(job.externalId)) continue;
        seen.add(job.externalId);
        jobs.push(job);
      }
    }
    source = citadelListingUrl(brand, "open-opportunities");
  } else {
    console.error(`no manual capture support for ${ats}`);
    process.exit(2);
  }

  if (jobs.length === 0) {
    console.error("parsed zero listings; check that the saved page is the listing page");
    process.exit(1);
  }
  const capture = toManualCapture(jobs, { source });
  const out = manualPath(ats);
  await fs.writeFile(out, `${JSON.stringify(capture, null, 2)}\n`);
  console.log(`wrote ${capture.jobs.length} jobs to ${out} (captured_at ${capture.captured_at})`);
}

await main();
