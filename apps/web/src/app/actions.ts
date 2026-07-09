"use server";

import { and, eq } from "drizzle-orm";
import { revalidatePath } from "next/cache";
import { savedJobs, savedSearches } from "@openintern/db";
import { auth } from "@/auth";
import { getDb } from "@/lib/db";

export async function saveJob(jobId: string) {
  const session = await auth();
  if (!session?.user?.id) throw new Error("Unauthorized");
  const db = getDb();
  await db
    .insert(savedJobs)
    .values({ userId: session.user.id, jobId })
    .onConflictDoNothing();
  revalidatePath("/account");
}

export async function unsaveJob(jobId: string) {
  const session = await auth();
  if (!session?.user?.id) throw new Error("Unauthorized");
  const db = getDb();
  await db
    .delete(savedJobs)
    .where(and(eq(savedJobs.userId, session.user.id), eq(savedJobs.jobId, jobId)));
  revalidatePath("/account");
}

export async function createSavedSearch(formData: FormData) {
  const session = await auth();
  if (!session?.user?.id) throw new Error("Unauthorized");
  const name = String(formData.get("name") || "My search").slice(0, 255);
  const query = String(formData.get("query") || "").trim() || null;
  const location = String(formData.get("location") || "").trim() || null;
  const companySlug = String(formData.get("company") || "").trim() || null;
  const remoteOnly = formData.get("remote") === "on";
  const webhookUrl = String(formData.get("webhook_url") || "").trim() || null;
  const emailEnabled = formData.get("email") !== "off";

  const db = getDb();
  await db.insert(savedSearches).values({
    userId: session.user.id,
    name,
    query,
    location,
    companySlug,
    remoteOnly,
    webhookUrl,
    emailEnabled,
  });
  revalidatePath("/account");
}

export async function deleteSavedSearch(id: string) {
  const session = await auth();
  if (!session?.user?.id) throw new Error("Unauthorized");
  const db = getDb();
  await db
    .delete(savedSearches)
    .where(and(eq(savedSearches.id, id), eq(savedSearches.userId, session.user.id)));
  revalidatePath("/account");
}
