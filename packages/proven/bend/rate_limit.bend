import Base

# Fixed-window rate limiter, one bucket per key. The TS wrapper owns the
# bucket map and the clock; this file decides every request.

type Bucket is Data:
  Bucket{count: Nat, reset_at: Nat}

type Verdict is Data:
  Verdict{ok: Bool, remaining: Nat, bucket: Bucket}

def ok(v: Verdict) -> Bool:
  match v:
    case Verdict{o, r, b}:
      o

def remaining(v: Verdict) -> Nat:
  match v:
    case Verdict{o, r, b}:
      r

def count(v: Verdict) -> Nat:
  match v:
    case Verdict{o, r, Bucket{c, t}}:
      c

def reset_at(v: Verdict) -> Nat:
  match v:
    case Verdict{o, r, Bucket{c, t}}:
      t

# First request of a new window. A limit of 0 admits nothing.
def fresh(now: Nat, limit: Nat, window: Nat) -> Verdict:
  match limit:
    case 0n:
      Verdict{False{}, 0n, Bucket{0n, Nat.add(now, window)}}
    case 1n+l:
      Verdict{True{}, l, Bucket{1n, Nat.add(now, window)}}

def admit.up(v: Verdict) -> Verdict:
  match v:
    case Verdict{o, r, Bucket{c, t}}:
      Verdict{o, r, Bucket{1n+c, t}}

# A request inside an open window: admitted while count < limit.
def admit(count: Nat, limit: Nat, reset_at: Nat) -> Verdict:
  match count limit:
    case 0n 0n:
      Verdict{False{}, 0n, Bucket{0n, reset_at}}
    case 0n 1n+l:
      Verdict{True{}, l, Bucket{1n, reset_at}}
    case 1n+c 0n:
      Verdict{False{}, 0n, Bucket{1n+c, reset_at}}
    case 1n+c 1n+l:
      admit.up(admit(c, l, reset_at))

def step.go(expired: Bool, count: Nat, reset_at: Nat, now: Nat, limit: Nat,
  window: Nat) -> Verdict:
  match expired:
    case True{}:
      fresh(now, limit, window)
    case False{}:
      admit(count, limit, reset_at)

def step(prev: Maybe<&2, Bucket>, +now: Nat, limit: Nat, window: Nat) -> Verdict:
  match prev:
    case None{}:
      fresh(now, limit, window)
    case Some{Bucket{c, +t}}:
      step.go(Nat.is_le(t, now), c, t, now, limit, window)

# The count a request leaves behind: one more if admitted, unchanged if not.
def bump(ok: Bool, count: Nat) -> Nat:
  match ok:
    case True{}:
      1n+count
    case False{}:
      count
