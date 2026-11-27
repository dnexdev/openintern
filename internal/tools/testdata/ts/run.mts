// Runs one TypeScript maintenance script against a recording.
//   tsx run.mts <script.ts> <recordings dir> <case.json> <result.json>
// fetch is replaced by the recording (first entry with the same method, URL
// and request body answers; anything unrecorded gets a 404), the clock is
// fixed, and stdout, stderr, the exit code and every request are written
// to result.json when the script finishes.
import fs from "node:fs";
import path from "node:path";
import zlib from "node:zlib";

const [scriptPath, recDir, casePath, resultPath] = process.argv.slice(2);
const tc = JSON.parse(fs.readFileSync(casePath, "utf8"));
const FIXED = Date.parse(tc.now ?? "2026-11-26T12:00:00.000Z");

const RealDate = Date;
class FixedDate extends RealDate {
  constructor(...args: any[]) {
    if (args.length === 0) super(FIXED);
    else super(...(args as [any]));
  }
  static now() {
    return FIXED;
  }
}
(globalThis as any).Date = FixedDate;

type Entry = {
  method: string; url: string; reqBody?: string; status?: number;
  headers?: Record<string, string>; body?: string; bodyFile?: string; error?: string;
};
const rec = JSON.parse(fs.readFileSync(path.join(recDir, tc.recording), "utf8"));
const entries: Entry[] = [...(tc.override ?? []), ...rec.responses];
const bodyOf = (e: Entry): string => {
  if (e.bodyFile) {
    const raw = fs.readFileSync(path.join(recDir, "..", e.bodyFile));
    return (e.bodyFile.endsWith(".gz") ? zlib.gunzipSync(raw) : raw).toString("utf8");
  }
  return e.body ?? "";
};
const requests: string[] = [];
(globalThis as any).fetch = async (input: any, init: any = {}) => {
  const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
  const method = (init.method ?? "GET").toUpperCase();
  const body = typeof init.body === "string" ? init.body : undefined;
  requests.push(`${method} ${url}${body !== undefined ? " " + body : ""}`);
  const e = entries.find(
    (x) => x.method === method && x.url === url && (x.reqBody ?? undefined) === body,
  );
  if (!e) return new Response("Not Found", { status: 404, headers: { "content-type": "text/plain" } });
  if (e.error) throw new TypeError("fetch failed");
  return new Response(bodyOf(e), { status: e.status ?? 200, headers: e.headers ?? {} });
};

let out = "";
let err = "";
process.stdout.write = ((chunk: any) => { out += String(chunk); return true; }) as any;
process.stderr.write = ((chunk: any) => { err += String(chunk); return true; }) as any;

let done = false;
function finish(code: number) {
  if (done) return;
  done = true;
  fs.writeFileSync(resultPath, JSON.stringify({ stdout: out, stderr: err, code, requests }, null, 1));
}
const realExit = process.exit.bind(process);
(process as any).exit = (code?: number) => {
  finish(code ?? (process.exitCode as number) ?? 0);
  realExit(code);
};
process.on("beforeExit", () => {
  finish((process.exitCode as number) ?? 0);
});

process.argv = [process.argv[0]!, scriptPath, ...tc.args];
await import(scriptPath);
