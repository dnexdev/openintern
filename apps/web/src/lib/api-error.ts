import { NextResponse } from "next/server";

export function apiDbError(err: unknown) {
  const message = err instanceof Error ? err.message : "Database unavailable";
  console.error("[api] database error:", err);
  return NextResponse.json(
    { error: "Database unavailable", detail: message },
    { status: 503, headers: { "Retry-After": "3600" } },
  );
}
