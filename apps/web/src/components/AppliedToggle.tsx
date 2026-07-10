"use client";

import { useEffect, useState, useSyncExternalStore } from "react";

const STORAGE_KEY = "openintern:applied";
/** Stable empty snapshot for SSR — must not allocate a new [] each call. */
const EMPTY_IDS: string[] = [];

function readIds(): string[] {
  if (typeof window === "undefined") return EMPTY_IDS;
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return EMPTY_IDS;
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed)
      ? parsed.filter((x): x is string => typeof x === "string")
      : EMPTY_IDS;
  } catch {
    return EMPTY_IDS;
  }
}

function getServerSnapshot(): string[] {
  return EMPTY_IDS;
}

function writeIds(ids: string[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(ids));
  window.dispatchEvent(new Event("openintern-applied"));
}

function subscribe(cb: () => void) {
  const handler = () => cb();
  window.addEventListener("openintern-applied", handler);
  window.addEventListener("storage", handler);
  return () => {
    window.removeEventListener("openintern-applied", handler);
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

export function AppliedToggle({ jobId }: { jobId: string }) {
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
