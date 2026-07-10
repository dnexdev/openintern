CREATE EXTENSION IF NOT EXISTS pgcrypto;--> statement-breakpoint
ALTER TABLE "jobs" RENAME COLUMN "external_id" TO "external_job_id";--> statement-breakpoint
ALTER TABLE "jobs" ADD COLUMN "fingerprint" varchar(64);--> statement-breakpoint
ALTER TABLE "jobs" ADD COLUMN "normalized_title" text DEFAULT '' NOT NULL;--> statement-breakpoint
ALTER TABLE "jobs" ADD COLUMN "role_family_id" text DEFAULT '' NOT NULL;--> statement-breakpoint
UPDATE "jobs"
SET "fingerprint" = encode(
  digest(("company_id"::text || '|' || "source" || '|' || "external_job_id"), 'sha256'),
  'hex'
)
WHERE "fingerprint" IS NULL;--> statement-breakpoint
ALTER TABLE "jobs" ALTER COLUMN "fingerprint" SET NOT NULL;--> statement-breakpoint
DROP INDEX IF EXISTS "jobs_company_external_uidx";--> statement-breakpoint
CREATE UNIQUE INDEX "jobs_fingerprint_uidx" ON "jobs" USING btree ("fingerprint");--> statement-breakpoint
CREATE INDEX "jobs_company_external_job_id_idx" ON "jobs" USING btree ("company_id","external_job_id");--> statement-breakpoint
CREATE INDEX "jobs_company_role_family_idx" ON "jobs" USING btree ("company_id","role_family_id");
