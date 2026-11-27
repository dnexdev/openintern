// TypeScript reference dump: dump-ts.mts <state-dir> <out-dir> <now-iso>
import { loadState } from "../../../packages/ingest/src/state.ts";
import { writeDumpsFromState } from "../../../packages/ingest/src/dump-state.ts";
const [stateDir, out, now] = process.argv.slice(2);
const r = await writeDumpsFromState(await loadState(stateDir), out, new Date(now));
console.log(`Wrote ${r.count} jobs to ${out}`);
