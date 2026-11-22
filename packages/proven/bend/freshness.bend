import Base

# Job freshness, shared by ingest (deactivate stale rows) and the web app
# (hide stale rows). Times are epoch milliseconds; the TS wrapper turns each
# {term, year} into the end-of-term instant using term_end below.

type Term is Data:
  Summer{}
  Fall{}
  Winter{}
  Other{}

# Last day of a term window: a zero-based month and a day of the month.
type MonthDay is Data:
  MonthDay{month0: Nat, day: Nat}

def term_end(t: Term) -> MonthDay:
  match t:
    case Summer{}:
      MonthDay{7n, 31n}
    case Fall{}:
      MonthDay{11n, 31n}
    case Winter{}:
      MonthDay{3n, 30n}
    case Other{}:
      MonthDay{11n, 31n}

def day_ms() -> Nat:
  86400000n

def max_age_days() -> Nat:
  120n

def last_seen_days() -> Nat:
  14n

# Some term window has not ended yet.
def any_live(ends: List<&2, Nat>, +now: Nat) -> Bool:
  match ends:
    case Nil{}:
      False{}
    case Con{h, t}:
      Bool.or(Nat.is_le(now, h), any_live(t, now))

# Every term window has ended.
def all_past(ends: List<&2, Nat>, +now: Nat) -> Bool:
  match ends:
    case Nil{}:
      True{}
    case Con{h, t}:
      Bool.and(Nat.is_lt(h, now), all_past(t, now))

# With term ends, a job lives while any term is live; without, for max_ms
# after its anchor (posted_at, else first_seen_at).
def is_fresh(ends: List<&2, Nat>, anchor: Nat, now: Nat, max_ms: Nat) -> Bool:
  match ends:
    case Nil{}:
      Nat.is_le(now, Nat.add(anchor, max_ms))
    case Con{h, t}:
      any_live(Con{h, t}, now)

def is_stale(ends: List<&2, Nat>, anchor: Nat, now: Nat, max_ms: Nat) -> Bool:
  match ends:
    case Nil{}:
      Nat.is_lt(Nat.add(anchor, max_ms), now)
    case Con{h, t}:
      all_past(Con{h, t}, now)

# Public rows also need at least one role tag.
def is_public(roles: Nat, ends: List<&2, Nat>, anchor: Nat, now: Nat,
  max_ms: Nat) -> Bool:
  match roles:
    case 0n:
      False{}
    case 1n+r:
      is_fresh(ends, anchor, now, max_ms)

# Not seen by any ingest run for longer than max_ms.
def stale_last_seen(seen: Nat, now: Nat, max_ms: Nat) -> Bool:
  Nat.is_lt(Nat.add(seen, max_ms), now)
