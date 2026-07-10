import {
  boolean,
  index,
  integer,
  jsonb,
  pgEnum,
  pgTable,
  text,
  timestamp,
  uniqueIndex,
  uuid,
  varchar,
} from "drizzle-orm/pg-core";

export const atsEnum = pgEnum("ats", [
  "greenhouse",
  "lever",
  "ashby",
  "workable",
  "smartrecruiters",
  "recruitee",
  "rippling",
  "bamboohr",
]);

export const companies = pgTable(
  "companies",
  {
    id: uuid("id").defaultRandom().primaryKey(),
    name: varchar("name", { length: 255 }).notNull(),
    slug: varchar("slug", { length: 255 }).notNull(),
    ats: atsEnum("ats").notNull(),
    boardToken: varchar("board_token", { length: 255 }).notNull(),
    careersUrl: text("careers_url"),
    websiteUrl: text("website_url"),
    active: boolean("active").notNull().default(true),
    createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp("updated_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    uniqueIndex("companies_slug_uidx").on(t.slug),
    index("companies_active_idx").on(t.active),
    index("companies_ats_token_idx").on(t.ats, t.boardToken),
  ],
);

export const jobs = pgTable(
  "jobs",
  {
    id: uuid("id").defaultRandom().primaryKey(),
    companyId: uuid("company_id")
      .notNull()
      .references(() => companies.id, { onDelete: "cascade" }),
    externalId: varchar("external_id", { length: 512 }).notNull(),
    title: varchar("title", { length: 512 }).notNull(),
    locations: jsonb("locations").$type<string[]>().notNull().default([]),
    applyUrl: text("apply_url").notNull(),
    excerpt: text("excerpt"),
    terms: jsonb("terms").$type<string[]>().notNull().default([]),
    termYears: jsonb("term_years")
      .$type<{ term: string; year: number }[]>()
      .notNull()
      .default([]),
    durationMonths: jsonb("duration_months").$type<number[]>().notNull().default([]),
    cohortYear: integer("cohort_year"),
    roles: jsonb("roles").$type<string[]>().notNull().default([]),
    regions: jsonb("regions").$type<string[]>().notNull().default([]),
    isRemote: boolean("is_remote").notNull().default(false),
    isActive: boolean("is_active").notNull().default(true),
    source: varchar("source", { length: 64 }).notNull(),
    postedAt: timestamp("posted_at", { withTimezone: true }),
    firstSeenAt: timestamp("first_seen_at", { withTimezone: true }).notNull().defaultNow(),
    lastSeenAt: timestamp("last_seen_at", { withTimezone: true }).notNull().defaultNow(),
    createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp("updated_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    uniqueIndex("jobs_company_external_uidx").on(t.companyId, t.externalId),
    index("jobs_active_idx").on(t.isActive),
    index("jobs_first_seen_idx").on(t.firstSeenAt),
    index("jobs_posted_at_idx").on(t.postedAt),
    index("jobs_title_idx").on(t.title),
  ],
);

export const ingestRuns = pgTable(
  "ingest_runs",
  {
    id: uuid("id").defaultRandom().primaryKey(),
    companyId: uuid("company_id").references(() => companies.id, { onDelete: "set null" }),
    status: varchar("status", { length: 32 }).notNull(),
    jobCount: integer("job_count").notNull().default(0),
    error: text("error"),
    ranAt: timestamp("ran_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    index("ingest_runs_company_idx").on(t.companyId),
    index("ingest_runs_ran_at_idx").on(t.ranAt),
  ],
);

export type Company = typeof companies.$inferSelect;
export type Job = typeof jobs.$inferSelect;
export type IngestRun = typeof ingestRuns.$inferSelect;
