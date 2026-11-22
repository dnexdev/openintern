# Incidents

One entry per incident, newest first. Times in UTC.

| Introduced | Detected | Contained | What | Cause | Follow-up |
|---|---|---|---|---|---|
| 2026-10-24 | 2026-10-24 | 2026-10-24 | Neon free compute quota exhausted; ingest and dump failed; API returned SQL text in 503s | Uncached health endpoints and DB-first reads kept compute awake | Dump-first reads, health from a published snapshot, state export runbook, move off Neon (plan phase D) |
| 2026-10 | 2026-10-18 | 2026-10-24 | Closed Khan Academy listing stayed up (#24) | Deactivation kept jobs when nothing on a non-empty board matched | Per-job retirement (laws dp_*), delisting list |
