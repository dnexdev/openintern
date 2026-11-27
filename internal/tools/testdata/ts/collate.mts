// Writes collate_node.json: names from community lists plus crafted cases,
// in input order and as Node's localeCompare sorts them (stable).
//   tsx collate.mts <packages/ingest/src> <list.md>... > collate_node.json
import fs from "node:fs";
const src = process.argv[2];
const { extractCompaniesFromMarkdown } = await import(src + "/gap-report.ts");
const { extractAtsBoardsFromMarkdown } = await import(src + "/parse-ats-url.ts");
const seen = new Set<string>();
for (const f of process.argv.slice(3)) {
  const md = fs.readFileSync(f, "utf8");
  for (const n of extractCompaniesFromMarkdown(md)) seen.add(n);
  for (const h of extractAtsBoardsFromMarkdown(md).hits) seen.add(h.companyName);
}
const crafted = [
  "resume", "Resume", "résumé", "Résumé", "RESUME", "resumes",
  "a b", "ab", "a-b", "a_b", "a.b", "a'b", "a’b", "a‘b", "a\"b", "a“b", "a”b", "a–b", "a—b",
  "BYTE/DEPT®", "BYTE/DEPT", "BYTE/DEPT©", "Lowe’s", "Lowe's", "Lowes", "Lowe s",
  "Léger", "Leger", "LEGER", "léger", "Lèger", "Légér",
  "Messe München", "Messe Munchen", "Messe Muenchen",
  "Clēnera", "Clenera", "AtkinsRéalis", "AtkinsRealis",
  "Société Générale", "Societe Generale", "bioMérieux", "BioMerieux",
  "Ünity", "Unity", "unity", "Øresund", "Oresund", "Ångström", "Angstrom",
  "🔥 Meta", "Meta", "🚀Rocket", "Rocket", "★Star", "Star", "→Arrow",
  "1Password", "3M", "2U", "10x Genomics", "0xPolygon", "100 Thieves",
  "(Stealth)", "[Stealth]", "@Home", "#Hash", "$Dollar", "&Co", "+Plus", "~Tilde",
  "ＦＵＬＬ", "FULL", "ﬁnance", "finance", "Ⅸ Corp", "IX Corp",
  "Δelta", "Delta", "Яндекс", "Yandex", "株式会社", "Kabushiki",
  "naïve", "naive", "Naive", "coöp", "coop", "co-op", "Co-op",
  "x", "X", "x ", "x.", "x-", "",
];
for (const c of crafted) seen.add(c);
const input = [...seen];
const sorted = [...input].sort((a, b) => a.localeCompare(b));
console.log(JSON.stringify({ input, sorted }, null, 1));
