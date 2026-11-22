import { NextResponse, type NextRequest } from "next/server";
import { refreshKillSwitch } from "@/lib/kill-switch";

/**
 * Maintenance mode: every page and API answers 503, except the uptime check.
 * Turn it on with OPENINTERN_MAINTENANCE=1 (needs a redeploy) or
 * "maintenance": true in the kill-switch file (no deploy, within minutes).
 */
export async function proxy(request: NextRequest) {
  if (request.nextUrl.pathname === "/api/v1/status") return NextResponse.next();
  const ks = await refreshKillSwitch();
  if (process.env.OPENINTERN_MAINTENANCE?.trim() !== "1" && !ks.maintenance) return NextResponse.next();

  const message = ks.message ?? "OpenIntern is down for maintenance. Please check back soon.";
  const headers = { "Cache-Control": "no-store", "Retry-After": "600" };
  if (request.nextUrl.pathname.startsWith("/api/")) {
    return NextResponse.json({ error: "Maintenance", message }, { status: 503, headers });
  }
  const escaped = message.replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`);
  return new NextResponse(
    `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta name="robots" content="noindex"><title>Maintenance · OpenIntern</title></head><body style="font-family:system-ui,sans-serif;max-width:36rem;margin:4rem auto;padding:0 1rem;line-height:1.5"><h1>Down for maintenance</h1><p>${escaped}</p></body></html>`,
    { status: 503, headers: { ...headers, "Content-Type": "text/html; charset=utf-8" } },
  );
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico|icon|apple-icon).*)"],
};
