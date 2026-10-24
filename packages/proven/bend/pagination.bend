import Base

# Positive-integer query params (page, limit). Results are written 1n+x so
# their lower bound is visible to the checker.

# The predecessor of a parsed positive value, or `fallback` when the text is
# missing, not a whole number, or zero.
def pred_or(m: Maybe<&2, Nat>, fallback: Nat) -> Nat:
  match m:
    case None{}:
      fallback
    case Some{n}:
      match n:
        case 0n:
          fallback
        case 1n+p:
          p

def parse(raw: Maybe<&2, String>) -> Maybe<&2, Nat>:
  match raw:
    case None{}:
      None{}
    case Some{s}:
      Nat.read(s)

# A value in [1, 1+max_pred], 1+default_pred when raw is unusable.
def clamp(raw: Maybe<&2, String>, default_pred: Nat, max_pred: Nat) -> Nat:
  1n+Nat.min(max_pred, pred_or(parse(raw), default_pred))

def page(raw: Maybe<&2, String>) -> Nat:
  clamp(raw, 0n, 99999n)

def limit(raw: Maybe<&2, String>) -> Nat:
  clamp(raw, 26n, 99n)

def offset(page: Nat, +limit: Nat) -> Nat:
  match page:
    case 0n:
      0n
    case 1n+p:
      Nat.mul(p, limit)
