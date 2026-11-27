import Base

# Dead apply links (internal/applycheck): a job's strike count after one
# probe of its apply link, and whether the count takes the job down.

# What one probe found: Alive = the posting answered; Dead = 404/410 or a
# redirect that lost the posting; Inconclusive = anything else (401/403/429,
# 5xx, timeout, login or bot wall).
type Verdict is Data:
  Alive{}
  Dead{}
  Inconclusive{}

# Strikes in a row that retire a job.
def strikes_to_retire() -> Nat:
  3n

# Least hours between two strikes that both count.
def strike_gap_hours() -> Nat:
  20n

def retired(count: Nat) -> Bool:
  Nat.is_le(strikes_to_retire(), count)

# A dead probe: a retired count stays; otherwise the first strike always
# counts and a later one only once the gap has passed.
def dead_step(done: Bool, count: Nat, due: Bool) -> Nat:
  match done:
    case True{}:
      count
    case False{}:
      match count:
        case 0n:
          1n
        case 1n+p:
          match due:
            case True{}:
              2n+p
            case False{}:
              1n+p

# count: strikes so far. due: at least strike_gap_hours() since the last
# strike (ignored when count is 0). A retired count stays as it is.
def next(+count: Nat, due: Bool, v: Verdict) -> Nat:
  match v:
    case Alive{}:
      0n
    case Inconclusive{}:
      count
    case Dead{}:
      dead_step(retired(count), count, due)
