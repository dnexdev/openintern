import {
  boolean,
  index,
  integer,
  jsonb,
  pgEnum,
  pgTable,
  primaryKey,
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
    durationMonths: integer("duration_months"),
    cohortYear: integer("cohort_year"),
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

/** Auth.js compatible user tables + app-specific saves/alerts */
export const users = pgTable("users", {
  id: text("id")
    .primaryKey()
    .$defaultFn(() => crypto.randomUUID()),
  name: text("name"),
  email: text("email").notNull().unique(),
  emailVerified: timestamp("email_verified", { withTimezone: true, mode: "date" }),
  image: text("image"),
  createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
});

export const accounts = pgTable(
  "accounts",
  {
    userId: text("user_id")
      .notNull()
      .references(() => users.id, { onDelete: "cascade" }),
    type: text("type").notNull(),
    provider: text("provider").notNull(),
    providerAccountId: text("provider_account_id").notNull(),
    refresh_token: text("refresh_token"),
    access_token: text("access_token"),
    expires_at: integer("expires_at"),
    token_type: text("token_type"),
    scope: text("scope"),
    id_token: text("id_token"),
    session_state: text("session_state"),
  },
  (t) => [
    primaryKey({ columns: [t.provider, t.providerAccountId] }),
  ],
);

export const sessions = pgTable("sessions", {
  sessionToken: text("session_token").primaryKey(),
  userId: text("user_id")
    .notNull()
    .references(() => users.id, { onDelete: "cascade" }),
  expires: timestamp("expires", { withTimezone: true, mode: "date" }).notNull(),
});

export const verificationTokens = pgTable(
  "verification_tokens",
  {
    identifier: text("identifier").notNull(),
    token: text("token").notNull(),
    expires: timestamp("expires", { withTimezone: true, mode: "date" }).notNull(),
  },
  (t) => [primaryKey({ columns: [t.identifier, t.token] })],
);

export const savedJobs = pgTable(
  "saved_jobs",
  {
    id: uuid("id").defaultRandom().primaryKey(),
    userId: text("user_id")
      .notNull()
      .references(() => users.id, { onDelete: "cascade" }),
    jobId: uuid("job_id")
      .notNull()
      .references(() => jobs.id, { onDelete: "cascade" }),
    createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [uniqueIndex("saved_jobs_user_job_uidx").on(t.userId, t.jobId)],
);

export const savedSearches = pgTable("saved_searches", {
  id: uuid("id").defaultRandom().primaryKey(),
  userId: text("user_id")
    .notNull()
    .references(() => users.id, { onDelete: "cascade" }),
  name: varchar("name", { length: 255 }).notNull(),
  query: varchar("query", { length: 512 }),
  location: varchar("location", { length: 255 }),
  companySlug: varchar("company_slug", { length: 255 }),
  remoteOnly: boolean("remote_only").notNull().default(false),
  webhookUrl: text("webhook_url"),
  emailEnabled: boolean("email_enabled").notNull().default(true),
  lastNotifiedAt: timestamp("last_notified_at", { withTimezone: true }),
  createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
});

export type Company = typeof companies.$inferSelect;
export type Job = typeof jobs.$inferSelect;
export type IngestRun = typeof ingestRuns.$inferSelect;
export type User = typeof users.$inferSelect;
export type SavedJob = typeof savedJobs.$inferSelect;
export type SavedSearch = typeof savedSearches.$inferSelect;
