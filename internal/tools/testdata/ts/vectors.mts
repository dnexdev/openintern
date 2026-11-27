// Writes vectors_node.json: the scripts' private helpers (slugify,
// normalizeName, the two tokenCandidates, the a16z anchor regex, the YC
// filter) on every name in collate_node.json and on generated HTML.
//   tsx vectors.mts <packages/ingest/src> <collate_node.json> > vectors_node.json
// The helpers are not exported, so this copies the two scripts with
// `export` added and main() removed, into a scratch copy of src.
import fs from "node:fs";
import path from "node:path";

const src = process.argv[2]!;
const scratch = "/tmp/openintern-tools/vectors-src";
fs.rmSync(scratch, { recursive: true, force: true });
fs.cpSync(src, scratch, { recursive: true });
const exportAll = (file: string, names: string[]) => {
  let text = fs.readFileSync(path.join(scratch, file), "utf8");
  for (const n of names) text = text.replace(`\nfunction ${n}(`, `\nexport function ${n}(`);
  text = text.replace(/\nmain\(\)\.catch\([\s\S]*$/, "\n");
  fs.writeFileSync(path.join(scratch, file), text);
};
exportAll("discover-accelerators.ts", ["tokenCandidates", "normalizeName", "isTechYc"]);
exportAll("recover-tokens.ts", ["tokenCandidates"]);
const disc = await import(path.join(scratch, "discover-accelerators.ts"));
const rec = await import(path.join(scratch, "recover-tokens.ts"));
const gap = await import(path.join(scratch, "gap-report.ts"));

const names: string[] = JSON.parse(fs.readFileSync(process.argv[3]!, "utf8")).input;
const extra = ["  Spaced  Out  ", "Under_score Co", "--Dashes--", "ℂomplex", "Ⅸ Corp", "İstanbul", "ΣΊΣΥΦΟΣ", "a b", "x y", "Tab\tCo", "émoji 🔥 co", "O'Reilly", "AT&T", "1", "", "a"];
const all = [...names, ...extra];
const websites = [undefined, null, "https://www.example.com", "http://sub.example.co.uk/x", "not a url", "https://a.io", "HTTPS://WWW.Upper.COM", "https://localhost:3000", "https://[::1]/", "https://xn--bcher-kva.example/"];
const people = all.map((name, i) => {
  const slug = gap.slugify(name);
  const website = websites[i % websites.length];
  const recAts = ["greenhouse", "smartrecruiters", "recruitee", "bamboohr"][i % 4];
  const recSlug = slug || "x";
  return {
    name,
    slug,
    norm: disc.normalizeName(name),
    website: website ?? null,
    discover: disc.tokenCandidates(slug || "hint", name, website),
    recAts,
    recSlug,
    recover: rec.tokenCandidates(recSlug, name, "Current.Token", recAts),
  };
});

// a16z anchors: the fixture page and generated markup around the regex's edges.
const re = /<a[^>]+href=["'](https?:\/\/(?!(?:www\.)?a16z\.com)[^"']+)["'][^>]*>([^<]{2,80})<\/a>/gi;
const parts = ["<a", "<A", " ", "href=", "HREF=", "\"", "'", "https://", "http://", "a16z.com", "www.a16z.com", "A16Z.COM/", "x.io", ">", "</a>", "</A>", "Co Name", "x", "<b>", "\n", "data-x ", "é", "𝔸"];
let seed = 42;
const rnd = (n: number) => { seed = (seed * 1103515245 + 12345) & 0x7fffffff; return (seed >>> 16) % n; };
const pick = <T,>(xs: T[]) => xs[rnd(xs.length)]!;
const htmls: string[] = [];
for (let i = 0; i < 600; i++) {
  // Structured: one or two anchors with every piece varied.
  let s = pick(["", "x ", "<div>", "<a>", "<a href='https://pre.example'>"]);
  for (let k = 0; k < 1 + rnd(2); k++) {
    const q = pick(['"', "'"]);
    const host = pick(["a16z.com", "www.a16z.com", "WWW.A16Z.COM", "a16z.company.io", "out.example", "www.out.example", "sub.a16z.com", "x"]);
    const attrs = pick(["", " ", " class=\"c\"", " data-href='y'", " href=\"https://www.a16z.com/z\" ", " a=\">\""]);
    const text = pick(["Co", "Some Company", "x", "", "  Spaced\n Name ", "Name <b>B</b>", "é".repeat(rnd(90)), "𝔸".repeat(rnd(45))]);
    const end = pick(["</a>", "</A>", "</ a>", "</abbr>", ""]);
    s += pick(["<a", "<A", "<abbr"]) + pick([" ", "  ", "\n", " id=1 ", ""]) + attrs + pick(["href=", "HREF=", "href ="]) + q +
      pick(["https://", "http://", "HTTPS://", "ftp://", "https:/"]) + host + pick(["", "/", "/p?q=1", "/a>b"]) + q + attrs + ">" + text + end;
    s += pick(["", " ", "\n", "<br>"]);
  }
  htmls.push(s);
}
for (let i = 0; i < 300; i++) {
  let s = "";
  const len = 3 + rnd(25);
  for (let j = 0; j < len; j++) s += parts[rnd(parts.length)];
  htmls.push(s);
}
htmls.push(
  '<a class="c" href="https://www.a16z.com/x" data-out href="https://www.out.example/">Out Co</a>',
  '<a href="https://out-two.example" data-x href="https://a16z.com/y">Out Two</a>',
  '<a href="https://u.example>still-url">Text Co</a>',
  '<a href="https://u.example">' + "y".repeat(80) + "</a>",
  '<a href="https://u.example">' + "y".repeat(81) + "</a>",
  '<a href="https://u.example">' + "😀".repeat(40) + "</a>",
  '<a href="https://u.example">' + "😀".repeat(41) + "</a>",
  '<abbr href="https://abbr.example">Abbr</abbr></a>',
  '<a href="https://a.example"><a href="https://b.example">Nested</a>',
);
const anchors = htmls.map((h) => ({ html: h, matches: [...h.matchAll(re)].map((m) => [m[1], m[2]]) }));

const yc = [
  { name: "A", status: "Inactive", industry: "B2B" },
  { name: "B", status: "Dead", industry: "B2B" },
  { name: "C", industry: "b2b software" },
  { name: "D", industry: "Government", tags: ["Developer Tools"] },
  { name: "E", industry: "Government", tags: ["Public Sector"] },
  { name: "F", industry: null, tags: [] },
  { name: "G", industry: "", tags: [] },
  { name: "H", industry: "Real Estate", tags: ["rail"] },
  { name: "I", status: "", industry: "Government" },
  { name: "J", industry: "Healthcare" },
  { name: "K", industry: "Government", tags: ["Climate", 5] },
  { name: "L", industry: "Government", tags: [null, "x"] },
  { name: "M", status: 0, industry: "Consumer" },
];
const ycOut = yc.map((c) => ({ company: c, tech: disc.isTechYc(c) }));

console.log(JSON.stringify({ people, anchors, yc: ycOut }, null, 1));
