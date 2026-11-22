/**
 * Runtime kill switch: a small JSON file the maintainer can edit from a phone
 * (in a separate repo, no deploy needed). The site re-reads it every few
 * minutes and hides any job, company or apply host it lists, or turns the
 * whole site into maintenance mode.
 *
 *   OPENINTERN_KILL_SWITCH_URL=https://raw.githubusercontent.com/<owner>/openintern-ops/main/killswitch.json
 *
 * {
 *   "jobs": ["<job uuid>"], "companies": ["<slug>"], "hosts": ["evil.example"],
 *   "maintenance": false, "message": "optional text for the maintenance page"
 * }
 *
 * If the file can't be read, the last good copy stays in force. Entries that
 * must outlive this file go in packages/db/src/delisted.ts.
 */

export type KillSwitch = {
  jobs: ReadonlySet<string>;
  companies: ReadonlySet<string>;
  hosts: ReadonlySet<string>;
  maintenance: boolean;
  message: string | null;
};

const EMPTY: KillSwitch = {
  jobs: new Set(),
  companies: new Set(),
  hosts: new Set(),
  maintenance: false,
  message: null,
};

/** How often the file is re-read. raw.githubusercontent.com adds up to 5 minutes of CDN caching. */
export const KILL_SWITCH_REFRESH_MS = 5 * 60 * 1000;

const state = globalThis as unknown as {
  __openinternKillSwitch?: { value: KillSwitch; checkedAt: number };
};

function strings(v: unknown, what: string): string[] {
  if (v == null) return [];
  if (!Array.isArray(v) || !v.every((x) => typeof x === "string")) {
    throw new Error(`kill switch: ${what} must be a list of strings`);
  }
  return v.map((x) => x.trim().toLowerCase()).filter(Boolean);
}

export function parseKillSwitch(data: unknown): KillSwitch {
  const o = (data ?? {}) as Record<string, unknown>;
  if (o.maintenance != null && typeof o.maintenance !== "boolean") {
    throw new Error("kill switch: maintenance must be true or false");
  }
  return {
    jobs: new Set(strings(o.jobs, "jobs")),
    companies: new Set(strings(o.companies, "companies")),
    hosts: new Set(strings(o.hosts, "hosts")),
    maintenance: o.maintenance === true,
    message: typeof o.message === "string" && o.message.trim() ? o.message.trim().slice(0, 500) : null,
  };
}

export function killSwitchUrl(): string | null {
  return process.env.OPENINTERN_KILL_SWITCH_URL?.trim() || null;
}

/** Re-read the file if it is due. Never throws; a failed read keeps the last good copy. */
export async function refreshKillSwitch(now = Date.now()): Promise<KillSwitch> {
  const url = killSwitchUrl();
  const cached = state.__openinternKillSwitch;
  if (!url) return cached?.value ?? EMPTY;
  if (cached && now - cached.checkedAt < KILL_SWITCH_REFRESH_MS) return cached.value;
  try {
    const res = await fetch(url, { cache: "no-store", signal: AbortSignal.timeout(3000) });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const value = parseKillSwitch(await res.json());
    state.__openinternKillSwitch = { value, checkedAt: now };
    return value;
  } catch (err) {
    console.warn("[kill-switch] keeping last good copy:", err instanceof Error ? err.message : err);
    const value = cached?.value ?? EMPTY;
    state.__openinternKillSwitch = { value, checkedAt: now };
    return value;
  }
}

/** The copy in force now (refreshKillSwitch must have run on this request path). */
export function currentKillSwitch(): KillSwitch {
  return state.__openinternKillSwitch?.value ?? EMPTY;
}

export function isKilled(companySlug: string, jobId: string, applyUrl: string, ks = currentKillSwitch()): boolean {
  if (ks.jobs.has(jobId.toLowerCase()) || ks.companies.has(companySlug.toLowerCase())) return true;
  if (ks.hosts.size === 0) return false;
  try {
    return ks.hosts.has(new URL(applyUrl).hostname.toLowerCase());
  } catch {
    return true;
  }
}
