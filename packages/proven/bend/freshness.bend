import Base

# Job freshness, shared by ingest (deactivate stale rows) and the web app
# (hide stale rows). Times are epoch milliseconds; the TS wrapper turns each
# {term, year} into the last instant a student can usefully apply, using
# apply_by below: the day before the term starts.

type Term is Data:
  Summer{}
  Fall{}
  Winter{}
  Spring{}
  Other{}

# The last day to apply for a term in a given year: `years_back` years
# earlier, zero-based month, day of month.
type ApplyBy is Data:
  ApplyBy{years_back: Nat, month0: Nat, day: Nat}

# The day before each term starts. Summer starts May 1, fall Sep 1, winter
# and spring (US semester) Jan 1 of the term's year. An unknown term keeps the
# whole year.
def apply_by(t: Term) -> ApplyBy:
  match t:
    case Summer{}:
      ApplyBy{0n, 3n, 30n}
    case Fall{}:
      ApplyBy{0n, 7n, 31n}
    case Winter{}:
      ApplyBy{1n, 11n, 31n}
    case Spring{}:
      ApplyBy{1n, 11n, 31n}
    case Other{}:
      ApplyBy{0n, 11n, 31n}

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
