import Base

# Ingest-health funnel totals. A run's error text is parsed in TS; missing
# counts arrive as None and count as zero.

type Funnel is Data:
  Funnel{fetched: Maybe<&2, Nat>, title: Maybe<&2, Nat>, tech: Maybe<&2, Nat>,
    upserted: Maybe<&2, Nat>, desc_missing: Maybe<&2, Nat>}

type Totals is Data:
  Totals{fetched: Nat, title: Nat, tech: Nat, upserted: Nat, desc_missing: Nat}

def or0(m: Maybe<&2, Nat>) -> Nat:
  match m:
    case None{}:
      0n
    case Some{n}:
      n

def zero() -> Totals:
  Totals{0n, 0n, 0n, 0n, 0n}

def of(f: Funnel) -> Totals:
  match f:
    case Funnel{a, b, c, d, e}:
      Totals{or0(a), or0(b), or0(c), or0(d), or0(e)}

def plus(x: Totals, y: Totals) -> Totals:
  match x y:
    case Totals{a, b, c, d, e} Totals{p, q, r, s, t}:
      Totals{Nat.add(a, p), Nat.add(b, q), Nat.add(c, r), Nat.add(d, s),
        Nat.add(e, t)}

def total(fs: List<&2, Funnel>) -> Totals:
  match fs:
    case Nil{}:
      zero()
    case Con{f, t}:
      plus(of(f), total(t))

def zero_match_tag() -> String:
  "zero_match:"

def is_zero_match(error: String) -> Bool:
  String.starts_with(error, zero_match_tag())
