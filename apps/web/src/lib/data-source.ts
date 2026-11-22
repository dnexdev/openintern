export type DataSource =
  | { kind: "db" }
  | { kind: "dump"; generatedAt: string };

export function dumpSource(generatedAt: string): DataSource {
  return { kind: "dump", generatedAt };
}

/**
 * Public board/API reads prefer the daily GitHub dump (zero Neon CU).
 * Set OPENINTERN_PREFER_DB=1 to restore live-DB-first reads for debugging.
 */
export function preferDbReads(): boolean {
  return process.env.OPENINTERN_PREFER_DB?.trim() === "1";
}

/** Dump-first with DB fallback, or DB-first with dump fallback when preferDbReads(). */
export async function withDumpFirst<T>(opts: {
  label: string;
  fromDump: () => Promise<T>;
  fromDb: () => Promise<T>;
}): Promise<T> {
  if (preferDbReads()) {
    try {
      return await opts.fromDb();
    } catch (dbErr) {
      console.warn(
        `[${opts.label}] database unavailable, falling back to dump:`,
        dbErr,
      );
      return opts.fromDump();
    }
  }
  try {
    return await opts.fromDump();
  } catch (dumpErr) {
    console.warn(
      `[${opts.label}] dump unavailable, falling back to database:`,
      dumpErr,
    );
    return opts.fromDb();
  }
}
