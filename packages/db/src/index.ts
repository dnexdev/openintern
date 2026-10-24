import { drizzle } from "drizzle-orm/postgres-js";
import postgres from "postgres";
// Extensionless so Next.js can resolve the TypeScript source when transpiling.
import * as schema from "./schema";

export function createDbClient(connectionString = process.env.DATABASE_URL) {
  if (!connectionString) {
    throw new Error("DATABASE_URL is required");
  }
  // Neon / PgBouncer: disable prepared statements; recycle idle sockets so
  // long ingest runs (many ATS HTTP waits) don't hit CONNECTION_CLOSED.
  const client = postgres(connectionString, {
    max: Number(process.env.OPENINTERN_DB_POOL_MAX ?? 5),
    idle_timeout: 20,
    max_lifetime: 60 * 10,
    connect_timeout: 30,
    prepare: false,
  });
  const db = drizzle(client, { schema });
  return { db, client };
}

export function createDb(connectionString = process.env.DATABASE_URL) {
  return createDbClient(connectionString).db;
}

export type Db = ReturnType<typeof createDb>;
export type SqlClient = ReturnType<typeof postgres>;

export * from "./schema";
export * from "./freshness";
export * from "./delisted";
