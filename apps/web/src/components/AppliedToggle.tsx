"use client";

import { useEffect, useState, useSyncExternalStore } from "react";

const STORAGE_KEY = "openintern:applied";
const HIDE_APPLIED_KEY = "openintern:hide-applied";
const STORE_EVENT = "openintern-applied";
/** Stable empty snapshot for SSR / empty store — must not allocate a new [] each call. */
const EMPTY_IDS: string[] = [];

/** Cached client snapshot so getSnapshot is referentially stable when contents unchanged. */
let cachedIds: string[] = EMPTY_IDS;
let cachedRaw: string | null = null;

function readIds(): string[] {
  if (typeof window === "undefined") return EMPTY_IDS;
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw === cachedRaw) return cachedIds;
    cachedRaw = raw;
    if (!raw) {
      cachedIds = EMPTY_IDS;
      return cachedIds;
    }
    const parsed = JSON.parse(raw);
    cachedIds = Array.isArray(parsed)
      ? parsed.filter((x): x is string => typeof x === "string")
      : EMPTY_IDS;
    return cachedIds;
  } catch {
    cachedRaw = null;
    cachedIds = EMPTY_IDS;
    return cachedIds;
  }
}

function getServerSnapshot(): string[] {
  return EMPTY_IDS;
}

function writeIds(ids: string[]) {
  const raw = JSON.stringify(ids);
  localStorage.setItem(STORAGE_KEY, raw);
  cachedRaw = raw;
  cachedIds = ids.length === 0 ? EMPTY_IDS : ids;
  window.dispatchEvent(new Event(STORE_EVENT));
}

function subscribe(cb: () => void) {
  const handler = () => cb();
  window.addEventListener(STORE_EVENT, handler);
  window.addEventListener("storage", handler);
  return () => {
    window.removeEventListener(STORE_EVENT, handler);
    window.removeEventListener("storage", handler);
  };
}

export function useAppliedIds(): string[] {
  return useSyncExternalStore(subscribe, readIds, getServerSnapshot);
}

export function useIsApplied(jobId: string): boolean {
  return useAppliedIds().includes(jobId);
}

export function toggleApplied(jobId: string) {
  const ids = readIds();
  if (ids.includes(jobId)) {
    writeIds(ids.filter((id) => id !== jobId));
  } else {
    writeIds([...ids, jobId]);
  }
}

function readHideApplied(): boolean {
  if (typeof window === "undefined") return false;
  return localStorage.getItem(HIDE_APPLIED_KEY) === "true";
}

export function useHideApplied(): [boolean, (value: boolean) => void] {
  const checked = useSyncExternalStore(subscribe, readHideApplied, () => false);
  const setChecked = (value: boolean) => {
    localStorage.setItem(HIDE_APPLIED_KEY, String(value));
    window.dispatchEvent(new Event(STORE_EVENT));
  };
  return [checked, setChecked];
}

export function AppliedToggle({
  jobId,
  jobTitle,
}: {
  jobId: string;
  jobTitle?: string;
}) {
  const applied = useIsApplied(jobId);
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);

  if (!mounted) {
    return (
      <button className="btn btn-sm" type="button" disabled>
        Mark applied
      </button>
    );
  }

  return (
    <button
      className={applied ? "btn btn-sm btn-primary" : "btn btn-sm"}
      type="button"
      onClick={() => toggleApplied(jobId)}
      aria-pressed={applied}
      aria-label={`${applied ? "Mark not applied" : "Mark applied"}${jobTitle ? `: ${jobTitle}` : ""}`}
    >
      {applied ? "Applied ✓" : "Mark applied"}
    </button>
  );
}

export function HideAppliedToggle({
  checked,
  onChange,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <label className="checkbox">
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
      />
      Hide applied
    </label>
  );
}
