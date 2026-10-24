import { NextResponse } from "next/server";

export function apiDbError(err: unknown) {
  console.error("[api] database error:", err);
  return NextResponse.json(
    { error: "Database unavailable" },
    { status: 503, headers: { "Retry-After": "3600", "Cache-Control": "no-store" } },
  );
}
