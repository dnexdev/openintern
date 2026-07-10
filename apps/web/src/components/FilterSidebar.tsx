"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, useTransition } from "react";

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

function toggleInList(list: string[], value: string, on: boolean) {
  if (on) return list.includes(value) ? list : [...list, value];
  return list.filter((x) => x !== value);
}

function toggleInNums(list: number[], value: number, on: boolean) {
  if (on) return list.includes(value) ? list : [...list, value];
  return list.filter((x) => x !== value);
}

function hrefFor(
  query: string,
  company: string,
  roles: string[],
  regions: string[],
  terms: string[],
  durations: number[],
  sort?: string,
) {
  const params = new URLSearchParams();
  if (query.trim()) params.set("q", query.trim());
  if (company) params.set("company", company);
  for (const r of roles) params.append("role", r);
  for (const r of regions) params.append("region", r);
  for (const t of terms) params.append("term", t);
  for (const d of durations) params.append("duration", String(d));
  if (sort && sort !== "first_seen") params.set("sort", sort);
  const qs = params.toString();
  return qs ? `/?${qs}` : "/";
}

export function FilterSidebar({
  query,
  company,
  companyOptions,
  roles,
  regions,
  terms,
  durations,
  hasFilters,
  sort,
}: {
  query: string;
  company: string;
  companyOptions: { slug: string; name: string }[];
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
  const [search, setSearch] = useState(query);
  useEffect(() => setSearch(query), [query]);
  const activeCount =
    Number(Boolean(query)) +
    Number(Boolean(company)) +
    roles.length +
    regions.length +
    terms.length +
    durations.length;

  function navigate(
    nextQuery: string,
    nextCompany: string,
    nextRoles: string[],
    nextRegions: string[],
    nextTerms: string[],
    nextDurations: number[],
  ) {
    const next = hrefFor(
      nextQuery,
      nextCompany,
      nextRoles,
      nextRegions,
      nextTerms,
      nextDurations,
      sort,
    );
    const current = hrefFor(query, company, roles, regions, terms, durations, sort);
    if (next === current) return;
    startTransition(() => {
      router.push(next, { scroll: false });
    });
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
        <form
          className="field"
          onSubmit={(e) => {
            e.preventDefault();
            navigate(search, company, roles, regions, terms, durations);
          }}
        >
          <label className="field-label" htmlFor="job-search">
            Search titles
          </label>
          <input
            id="job-search"
            className="input"
            type="search"
            value={search}
            placeholder="Software intern"
            onChange={(e) => setSearch(e.target.value)}
          />
          <button className="btn btn-sm" type="submit">
            Search
          </button>
        </form>
        <div className="field">
          <label className="field-label" htmlFor="company-filter">
            Company
          </label>
          <select
            id="company-filter"
            value={company}
            onChange={(e) =>
              navigate(query, e.target.value, roles, regions, terms, durations)
            }
          >
            <option value="">All companies</option>
            {companyOptions.map((option) => (
              <option key={option.slug} value={option.slug}>
                {option.name}
              </option>
            ))}
          </select>
        </div>
        <div className="field">
          <span className="field-label">Role</span>
          <div className="chip-grid">
            {ROLE_OPTIONS.map((r) => (
              <label key={r} className="checkbox">
                <input
                  type="checkbox"
                  checked={roles.includes(r)}
                  onChange={(e) =>
                    navigate(
                      query,
                      company,
                      toggleInList(roles, r, e.target.checked),
                      regions,
                      terms,
                      durations,
                    )
                  }
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
                checked={regions.includes(r.value)}
                onChange={(e) =>
                  navigate(
                    query,
                    company,
                    roles,
                    toggleInList(regions, r.value, e.target.checked),
                    terms,
                    durations,
                  )
                }
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
                checked={terms.includes(t)}
                onChange={(e) =>
                  navigate(
                    query,
                    company,
                    roles,
                    regions,
                    toggleInList(terms, t, e.target.checked),
                    durations,
                  )
                }
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
                checked={durations.includes(m)}
                onChange={(e) =>
                  navigate(
                    query,
                    company,
                    roles,
                    regions,
                    terms,
                    toggleInNums(durations, m, e.target.checked),
                  )
                }
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
      </div>
    </aside>
  );
}
