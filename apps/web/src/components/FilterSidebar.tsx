"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, useTransition, type FormEvent } from "react";

const ROLE_OPTIONS = [
  "software",
  "backend",
  "frontend",
  "fullstack",
  "data",
  "ml",
  "mobile",
  "security",
  "devops",
  "hardware",
  "quant",
  "product",
  "research",
] as const;

const REGION_OPTIONS = [
  { value: "remote", label: "Remote" },
  { value: "us", label: "US" },
  { value: "canada", label: "Canada" },
  { value: "europe", label: "UK/Europe" },
  { value: "other", label: "Other" },
] as const;

const TERM_OPTIONS = ["summer", "fall", "winter"] as const;
const DURATION_OPTIONS = [3, 4, 6, 8, 12] as const;

function capitalize(s: string) {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

export function FilterSidebar({
  roles,
  regions,
  terms,
  durations,
  hasFilters,
  sort,
}: {
  roles: string[];
  regions: string[];
  terms: string[];
  durations: number[];
  hasFilters: boolean;
  sort?: string;
}) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [open, setOpen] = useState(false);
  const activeCount = roles.length + regions.length + terms.length + durations.length;

  function applyFilters(form: HTMLFormElement) {
    const data = new FormData(form);
    const params = new URLSearchParams();
    for (const [key, value] of data.entries()) {
      if (typeof value === "string" && value) params.append(key, value);
    }
    if (sort && sort !== "first_seen") params.set("sort", sort);
    const qs = params.toString();
    startTransition(() => {
      router.push(qs ? `/?${qs}` : "/");
    });
  }

  function onChange(e: FormEvent<HTMLFormElement>) {
    applyFilters(e.currentTarget);
  }

  return (
    <aside className={`sidebar${pending ? " is-pending" : ""}${open ? " is-open" : ""}`}>
      <button
        type="button"
        className="btn sidebar-toggle"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      >
        Filters{activeCount > 0 ? ` (${activeCount})` : ""}
      </button>
      <div className="sidebar-body">
        <h2>Filters</h2>
        <form method="get" action="/" onChange={onChange}>
          <div className="field">
            <span className="field-label">Role</span>
            <div className="chip-grid">
              {ROLE_OPTIONS.map((r) => (
                <label key={r} className="checkbox">
                  <input
                    type="checkbox"
                    name="role"
                    value={r}
                    defaultChecked={roles.includes(r)}
                  />
                  {r}
                </label>
              ))}
            </div>
          </div>
          <div className="field">
            <span className="field-label">Location</span>
            {REGION_OPTIONS.map((r) => (
              <label key={r.value} className="checkbox">
                <input
                  type="checkbox"
                  name="region"
                  value={r.value}
                  defaultChecked={regions.includes(r.value)}
                />
                {r.label}
              </label>
            ))}
          </div>
          <div className="field">
            <span className="field-label">Term</span>
            {TERM_OPTIONS.map((t) => (
              <label key={t} className="checkbox">
                <input
                  type="checkbox"
                  name="term"
                  value={t}
                  defaultChecked={terms.includes(t)}
                />
                {capitalize(t)}
              </label>
            ))}
          </div>
          <div className="field">
            <span className="field-label">Duration</span>
            {DURATION_OPTIONS.map((m) => (
              <label key={m} className="checkbox">
                <input
                  type="checkbox"
                  name="duration"
                  value={m}
                  defaultChecked={durations.includes(m)}
                />
                {m} mo{m === 3 ? " (summer)" : ""}
              </label>
            ))}
          </div>
          {hasFilters ? (
            <div className="sidebar-actions">
              <Link className="btn" href="/">
                Clear
              </Link>
            </div>
          ) : null}
        </form>
      </div>
    </aside>
  );
}
