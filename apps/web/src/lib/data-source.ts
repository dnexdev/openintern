export type DataSource =
  | { kind: "db" }
  | { kind: "dump"; generatedAt: string };

export function dumpSource(generatedAt: string): DataSource {
  return { kind: "dump", generatedAt };
}
