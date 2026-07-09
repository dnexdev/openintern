import { drizzle } from "drizzle-orm/postgres-js";
import postgres from "postgres";
// Extensionless so Next.js can resolve the TypeScript source when transpiling.
import * as schema from "./schema";

export function createDbClient(connectionString = process.env.DATABASE_URL) {
  if (!connectionString) {
    throw new Error("DATABASE_URL is required");
  }
  const client = postgres(connectionString, { max: 10 });
  const db = drizzle(client, { schema });
  return { db, client };
}

export function createDb(connectionString = process.env.DATABASE_URL) {
  return createDbClient(connectionString).db;
}

export type Db = ReturnType<typeof createDb>;
export type SqlClient = ReturnType<typeof postgres>;

export * from "./schema";
