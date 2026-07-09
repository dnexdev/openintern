import { createDb, type Db } from "@openintern/db";

const globalForDb = globalThis as unknown as { __openinternDb?: Db };

export function getDb(): Db {
  if (!globalForDb.__openinternDb) {
    globalForDb.__openinternDb = createDb(process.env.DATABASE_URL);
  }
  return globalForDb.__openinternDb;
}
