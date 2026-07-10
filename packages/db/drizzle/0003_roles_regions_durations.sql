-- Convert duration_months from nullable int → jsonb int[]
ALTER TABLE "jobs" ADD COLUMN IF NOT EXISTS "duration_months_new" jsonb DEFAULT '[]'::jsonb NOT NULL;--> statement-breakpoint
UPDATE "jobs" SET "duration_months_new" = CASE
  WHEN "duration_months" IS NULL THEN '[]'::jsonb
  ELSE jsonb_build_array("duration_months")
END;--> statement-breakpoint
ALTER TABLE "jobs" DROP COLUMN IF EXISTS "duration_months";--> statement-breakpoint
ALTER TABLE "jobs" RENAME COLUMN "duration_months_new" TO "duration_months";--> statement-breakpoint
ALTER TABLE "jobs" ADD COLUMN IF NOT EXISTS "roles" jsonb DEFAULT '[]'::jsonb NOT NULL;--> statement-breakpoint
ALTER TABLE "jobs" ADD COLUMN IF NOT EXISTS "regions" jsonb DEFAULT '[]'::jsonb NOT NULL;--> statement-breakpoint
ALTER TABLE "jobs" ADD COLUMN IF NOT EXISTS "term_years" jsonb DEFAULT '[]'::jsonb NOT NULL;
