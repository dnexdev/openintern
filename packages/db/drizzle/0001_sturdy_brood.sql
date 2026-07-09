ALTER TYPE "public"."ats" ADD VALUE IF NOT EXISTS 'workable';--> statement-breakpoint
ALTER TYPE "public"."ats" ADD VALUE IF NOT EXISTS 'smartrecruiters';--> statement-breakpoint
ALTER TABLE "jobs" ADD COLUMN IF NOT EXISTS "terms" jsonb DEFAULT '[]'::jsonb NOT NULL;--> statement-breakpoint
ALTER TABLE "jobs" ADD COLUMN IF NOT EXISTS "duration_months" integer;
