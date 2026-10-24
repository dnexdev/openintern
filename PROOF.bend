import Base
import ./LAWS.bend as Laws
import ./packages/proven/bend/lemmas.bend as L
import ./packages/proven/bend/rate_limit.bend as RL
import ./packages/proven/bend/freshness.bend as FR
import ./packages/proven/bend/apply_picker.bend as AP
import ./packages/proven/bend/deactivation.bend as DP
import ./packages/proven/bend/funnel.bend as FN
import ./packages/proven/bend/pagination.bend as PG
import ./packages/proven/bend/job_slug.bend as JS

# PROOF.bend: a proof for every law in LAWS.bend. `bend PROOF.bend` is the
# gate; the compiled module (packages/proven/dist/proven.mjs) is built from
# this file, so the app runs exactly the code proven here.

# ---------------------------------------------------------------------------
# Rate limiter
# ---------------------------------------------------------------------------

law rl.up_ok:
  for +v: RL.Verdict
  {RL.ok(v) == RL.ok(RL.admit.up(v)) : Bool}

def rl.up_ok(v):
  match v:
    case RL.Verdict{o, r, RL.Bucket{c, t}}:
      {==}

law rl.up_rem:
  for +v: RL.Verdict
  {RL.remaining(v) == RL.remaining(RL.admit.up(v)) : Nat}

def rl.up_rem(v):
  match v:
    case RL.Verdict{o, r, RL.Bucket{c, t}}:
      {==}

law rl.up_count:
  for +v: RL.Verdict
  {1n+RL.count(v) == RL.count(RL.admit.up(v)) : Nat}

def rl.up_count(v):
  match v:
    case RL.Verdict{o, r, RL.Bucket{c, t}}:
      {==}

law rl.up_reset:
  for +v: RL.Verdict
  {RL.reset_at(v) == RL.reset_at(RL.admit.up(v)) : Nat}

def rl.up_reset(v):
  match v:
    case RL.Verdict{o, r, RL.Bucket{c, t}}:
      {==}

law rl.bump_succ:
  for +b: Bool
  for +n: Nat
  {1n+RL.bump(b, n) == RL.bump(b, 1n+n) : Nat}

def rl.bump_succ(b, n):
  match b:
    case True{}:
      {==}
    case False{}:
      {==}

law rl.admit_rem:
  for +c: Nat
  for +l: Nat
  for +r: Nat
  {RL.remaining(RL.admit(c, l, r)) == Nat.sub(l, RL.count(RL.admit(c, l, r))) : Nat}

def rl.admit_rem(c, l, r):
  match c l:
    case 0n 0n:
      {==}
    case 0n 1n+m:
      Equal.sym(Nat, Nat.sub(m, 0n), m, L.sub_zero(m))
    case 1n+d 0n:
      {==}
    case 1n+d 1n+m:
      %rl.up_rem(RL.admit(d, m, r)) :
        {_ == Nat.sub(1n+m, RL.count(RL.admit.up(RL.admit(d, m, r)))) : Nat}
      %rl.up_count(RL.admit(d, m, r)) :
        {RL.remaining(RL.admit(d, m, r)) == Nat.sub(1n+m, _) : Nat}
      rl.admit_rem(d, m, r)

law rl.admit_bounded:
  for +c: Nat
  for +l: Nat
  for +r: Nat
  {Bool.or(Bool.not(RL.ok(RL.admit(c, l, r))),
    Nat.is_le(RL.count(RL.admit(c, l, r)), l)) == True{} : Bool}

def rl.admit_bounded(c, l, r):
  match c l:
    case 0n 0n:
      {==}
    case 0n 1n+m:
      L.le_one_succ(m)
    case 1n+d 0n:
      {==}
    case 1n+d 1n+m:
      %rl.up_ok(RL.admit(d, m, r)) :
        {Bool.or(Bool.not(_),
          Nat.is_le(RL.count(RL.admit.up(RL.admit(d, m, r))), 1n+m)) == True{} : Bool}
      %rl.up_count(RL.admit(d, m, r)) :
        {Bool.or(Bool.not(RL.ok(RL.admit(d, m, r))), Nat.is_le(_, 1n+m)) == True{} : Bool}
      rl.admit_bounded(d, m, r)

law rl.admit_zero:
  for +c: Nat
  for +r: Nat
  {RL.ok(RL.admit(c, 0n, r)) == False{} : Bool}

def rl.admit_zero(c, r):
  match c:
    case 0n:
      {==}
    case 1n+d:
      {==}

law rl.admit_count:
  for +c: Nat
  for +l: Nat
  for +r: Nat
  {RL.count(RL.admit(c, l, r)) == RL.bump(RL.ok(RL.admit(c, l, r)), c) : Nat}

def rl.admit_count(c, l, r):
  match c l:
    case 0n 0n:
      {==}
    case 0n 1n+m:
      {==}
    case 1n+d 0n:
      {==}
    case 1n+d 1n+m:
      %rl.up_count(RL.admit(d, m, r)) :
        {_ == RL.bump(RL.ok(RL.admit.up(RL.admit(d, m, r))), 1n+d) : Nat}
      %rl.up_ok(RL.admit(d, m, r)) :
        {1n+RL.count(RL.admit(d, m, r)) == RL.bump(_, 1n+d) : Nat}
      %rl.bump_succ(RL.ok(RL.admit(d, m, r)), d) :
        {1n+RL.count(RL.admit(d, m, r)) == _ : Nat}
      %rl.admit_count(d, m, r) :
        {1n+RL.count(RL.admit(d, m, r)) == 1n+_ : Nat}
      {==}

law rl.admit_reset:
  for +c: Nat
  for +l: Nat
  for +r: Nat
  {RL.reset_at(RL.admit(c, l, r)) == r : Nat}

def rl.admit_reset(c, l, r):
  match c l:
    case 0n 0n:
      {==}
    case 0n 1n+m:
      {==}
    case 1n+d 0n:
      {==}
    case 1n+d 1n+m:
      %rl.up_reset(RL.admit(d, m, r)) : {_ == r : Nat}
      rl.admit_reset(d, m, r)

law rl.fresh_rem:
  for +now: Nat
  for +l: Nat
  for +w: Nat
  {RL.remaining(RL.fresh(now, l, w)) == Nat.sub(l, RL.count(RL.fresh(now, l, w))) : Nat}

def rl.fresh_rem(now, l, w):
  match l:
    case 0n:
      {==}
    case 1n+m:
      Equal.sym(Nat, Nat.sub(m, 0n), m, L.sub_zero(m))

law rl.fresh_bounded:
  for +now: Nat
  for +l: Nat
  for +w: Nat
  {Bool.or(Bool.not(RL.ok(RL.fresh(now, l, w))),
    Nat.is_le(RL.count(RL.fresh(now, l, w)), l)) == True{} : Bool}

def rl.fresh_bounded(now, l, w):
  match l:
    case 0n:
      {==}
    case 1n+m:
      L.le_one_succ(m)

law rl.go_rem:
  for +e: Bool
  for +c: Nat
  for +r: Nat
  for +now: Nat
  for +l: Nat
  for +w: Nat
  {RL.remaining(RL.step.go(e, c, r, now, l, w))
    == Nat.sub(l, RL.count(RL.step.go(e, c, r, now, l, w))) : Nat}

def rl.go_rem(e, c, r, now, l, w):
  match e:
    case True{}:
      rl.fresh_rem(now, l, w)
    case False{}:
      rl.admit_rem(c, l, r)

law rl.go_bounded:
  for +e: Bool
  for +c: Nat
  for +r: Nat
  for +now: Nat
  for +l: Nat
  for +w: Nat
  {Bool.or(Bool.not(RL.ok(RL.step.go(e, c, r, now, l, w))),
    Nat.is_le(RL.count(RL.step.go(e, c, r, now, l, w)), l)) == True{} : Bool}

def rl.go_bounded(e, c, r, now, l, w):
  match e:
    case True{}:
      rl.fresh_bounded(now, l, w)
    case False{}:
      rl.admit_bounded(c, l, r)

law rl.go_zero:
  for +e: Bool
  for +c: Nat
  for +r: Nat
  for +now: Nat
  for +w: Nat
  {RL.ok(RL.step.go(e, c, r, now, 0n, w)) == False{} : Bool}

def rl.go_zero(e, c, r, now, w):
  match e:
    case True{}:
      {==}
    case False{}:
      rl.admit_zero(c, r)

def Laws.rl_remaining(prev, now, limit, window):
  match prev:
    case None{}:
      rl.fresh_rem(now, limit, window)
    case Some{RL.Bucket{c, t}}:
      rl.go_rem(Nat.is_le(t, now), c, t, now, limit, window)

def Laws.rl_bounded(prev, now, limit, window):
  match prev:
    case None{}:
      rl.fresh_bounded(now, limit, window)
    case Some{RL.Bucket{c, t}}:
      rl.go_bounded(Nat.is_le(t, now), c, t, now, limit, window)

def Laws.rl_zero_limit(prev, now, window):
  match prev:
    case None{}:
      {==}
    case Some{RL.Bucket{c, t}}:
      rl.go_zero(Nat.is_le(t, now), c, t, now, window)

def Laws.rl_expired(count, reset_at, k, limit, window):
  %Equal.sym(Bool, Nat.is_le(reset_at, Nat.add(reset_at, k)), True{},
    L.le_add(reset_at, k)) :
    {RL.step.go(_, count, reset_at, Nat.add(reset_at, k), limit, window)
      == RL.fresh(Nat.add(reset_at, k), limit, window) : RL.Verdict}
  {==}

def Laws.rl_open_window(count, now, k, limit, window):
  %Equal.sym(Bool, Nat.is_le(1n+Nat.add(now, k), now), False{},
    L.lt_add_succ(now, k)) :
    {RL.step.go(_, count, 1n+Nat.add(now, k), now, limit, window)
      == RL.admit(count, limit, 1n+Nat.add(now, k)) : RL.Verdict}
  {==}

def Laws.rl_admit_count(count, limit, reset_at):
  rl.admit_count(count, limit, reset_at)

def Laws.rl_admit_keeps_window(count, limit, reset_at):
  rl.admit_reset(count, limit, reset_at)

def Laws.rl_fresh_window(now, limit, window):
  match limit:
    case 0n:
      {==}
    case 1n+m:
      {==}

# ---------------------------------------------------------------------------
# Freshness
# ---------------------------------------------------------------------------

law fr.past_live:
  for +xs: List<&2, Nat>
  for +n: Nat
  {FR.all_past(xs, n) == Bool.not(FR.any_live(xs, n)) : Bool}

def fr.past_live(xs, n):
  match xs:
    case Nil{}:
      {==}
    case Con{h, t}:
      %Equal.sym(Bool, Bool.not(Bool.or(Nat.is_le(n, h), FR.any_live(t, n))),
        Bool.and(Bool.not(Nat.is_le(n, h)), Bool.not(FR.any_live(t, n))),
        L.not_or(Nat.is_le(n, h), FR.any_live(t, n))) :
        {Bool.and(Nat.is_lt(h, n), FR.all_past(t, n)) == _ : Bool}
      %Equal.sym(Bool, Nat.is_lt(h, n), Bool.not(Nat.is_le(n, h)), L.lt_not_le(h, n)) :
        {Bool.and(_, FR.all_past(t, n))
          == Bool.and(Bool.not(Nat.is_le(n, h)), Bool.not(FR.any_live(t, n))) : Bool}
      %Equal.sym(Bool, FR.all_past(t, n), Bool.not(FR.any_live(t, n)), fr.past_live(t, n)) :
        {Bool.and(Bool.not(Nat.is_le(n, h)), _)
          == Bool.and(Bool.not(Nat.is_le(n, h)), Bool.not(FR.any_live(t, n))) : Bool}
      {==}

law fr.all_past_mono:
  for +xs: List<&2, Nat>
  for +n: Nat
  for +k: Nat
  for +e: {FR.all_past(xs, n) == True{} : Bool}
  {FR.all_past(xs, Nat.add(n, k)) == True{} : Bool}

def fr.all_past_mono(xs, n, k, e):
  match xs:
    case Nil{}:
      {==}
    case Con{h, t}:
      L.and_intro(Nat.is_lt(h, Nat.add(n, k)), FR.all_past(t, Nat.add(n, k)),
        L.lt_mono(h, n, k, L.and_l(Nat.is_lt(h, n), FR.all_past(t, n), e)),
        fr.all_past_mono(t, n, k, L.and_r(Nat.is_lt(h, n), FR.all_past(t, n), e)))

def Laws.fr_stale_iff_not_fresh(ends, anchor, now, max_ms):
  match ends:
    case Nil{}:
      L.lt_not_le(Nat.add(anchor, max_ms), now)
    case Con{h, t}:
      fr.past_live(Con{h, t}, now)

def Laws.fr_stale_monotone(ends, anchor, now, k, max_ms, e):
  match ends:
    case Nil{}:
      L.lt_mono(Nat.add(anchor, max_ms), now, k, e)
    case Con{h, t}:
      fr.all_past_mono(Con{h, t}, now, k, e)

def Laws.fr_last_seen_monotone(seen, now, k, max_ms, e):
  L.lt_mono(Nat.add(seen, max_ms), now, k, e)

def Laws.fr_term_live_at_end(h, t, anchor, max_ms):
  %Equal.sym(Bool, Nat.is_le(h, h), True{}, L.le_refl(h)) :
    {Bool.or(_, FR.any_live(t, h)) == True{} : Bool}
  {==}

def Laws.fr_age_live_at_limit(anchor, max_ms):
  L.le_refl(Nat.add(anchor, max_ms))

def Laws.fr_public_needs_role(ends, anchor, now, max_ms):
  {==}

def Laws.fr_public_is_fresh(roles, ends, anchor, now, max_ms):
  {==}

def Laws.fr_summer_end():
  {==}

def Laws.fr_fall_end():
  {==}

def Laws.fr_winter_end():
  {==}

def Laws.fr_other_end():
  {==}

# ---------------------------------------------------------------------------
# Apply picker
# ---------------------------------------------------------------------------

def Laws.ap_empty(locations):
  {==}

def Laws.ap_many_postings(p, locations):
  {==}

def Laws.ap_one_posting(locations):
  {==}

def Laws.ap_title_matches_picker(locations):
  {==}

def Laws.ap_distinct_says_locations(n):
  L.lt_irrefl(n)

def Laws.ap_repeats_say_openings(unique, k):
  L.lt_succ_add(unique, k)

# ---------------------------------------------------------------------------
# Deactivation policy
# ---------------------------------------------------------------------------

def Laws.dp_failed_fetch_keeps(fetched, tech):
  {==}

def Laws.dp_matches_retire_missing(fetched, tech):
  match fetched:
    case 0n:
      {==}
    case 1n+f:
      {==}

def Laws.dp_empty_board_retires_all():
  {==}

def Laws.dp_zero_match_keeps(fetched):
  {==}

# ---------------------------------------------------------------------------
# Ingest-health funnel
# ---------------------------------------------------------------------------

law fn.zero_plus:
  for +t: FN.Totals
  {t == FN.plus(FN.zero(), t) : FN.Totals}

def fn.zero_plus(t):
  match t:
    case FN.Totals{a, b, c, d, e}:
      {==}

law fn.plus_assoc:
  for +x: FN.Totals
  for +y: FN.Totals
  for +z: FN.Totals
  {FN.plus(FN.plus(x, y), z) == FN.plus(x, FN.plus(y, z)) : FN.Totals}

def fn.plus_assoc(x, y, z):
  match x y z:
    case FN.Totals{a1, a2, a3, a4, a5} FN.Totals{b1, b2, b3, b4, b5}
      FN.Totals{c1, c2, c3, c4, c5}:
      %L.add_assoc(a1, b1, c1) :
        {FN.Totals{Nat.add(Nat.add(a1, b1), c1), Nat.add(Nat.add(a2, b2), c2),
          Nat.add(Nat.add(a3, b3), c3), Nat.add(Nat.add(a4, b4), c4),
          Nat.add(Nat.add(a5, b5), c5)}
        == FN.Totals{_, Nat.add(a2, Nat.add(b2, c2)), Nat.add(a3, Nat.add(b3, c3)),
          Nat.add(a4, Nat.add(b4, c4)), Nat.add(a5, Nat.add(b5, c5))} : FN.Totals}
      %L.add_assoc(a2, b2, c2) :
        {FN.Totals{Nat.add(Nat.add(a1, b1), c1), Nat.add(Nat.add(a2, b2), c2),
          Nat.add(Nat.add(a3, b3), c3), Nat.add(Nat.add(a4, b4), c4),
          Nat.add(Nat.add(a5, b5), c5)}
        == FN.Totals{Nat.add(Nat.add(a1, b1), c1), _, Nat.add(a3, Nat.add(b3, c3)),
          Nat.add(a4, Nat.add(b4, c4)), Nat.add(a5, Nat.add(b5, c5))} : FN.Totals}
      %L.add_assoc(a3, b3, c3) :
        {FN.Totals{Nat.add(Nat.add(a1, b1), c1), Nat.add(Nat.add(a2, b2), c2),
          Nat.add(Nat.add(a3, b3), c3), Nat.add(Nat.add(a4, b4), c4),
          Nat.add(Nat.add(a5, b5), c5)}
        == FN.Totals{Nat.add(Nat.add(a1, b1), c1), Nat.add(Nat.add(a2, b2), c2), _,
          Nat.add(a4, Nat.add(b4, c4)), Nat.add(a5, Nat.add(b5, c5))} : FN.Totals}
      %L.add_assoc(a4, b4, c4) :
        {FN.Totals{Nat.add(Nat.add(a1, b1), c1), Nat.add(Nat.add(a2, b2), c2),
          Nat.add(Nat.add(a3, b3), c3), Nat.add(Nat.add(a4, b4), c4),
          Nat.add(Nat.add(a5, b5), c5)}
        == FN.Totals{Nat.add(Nat.add(a1, b1), c1), Nat.add(Nat.add(a2, b2), c2),
          Nat.add(Nat.add(a3, b3), c3), _, Nat.add(a5, Nat.add(b5, c5))} : FN.Totals}
      %L.add_assoc(a5, b5, c5) :
        {FN.Totals{Nat.add(Nat.add(a1, b1), c1), Nat.add(Nat.add(a2, b2), c2),
          Nat.add(Nat.add(a3, b3), c3), Nat.add(Nat.add(a4, b4), c4),
          Nat.add(Nat.add(a5, b5), c5)}
        == FN.Totals{Nat.add(Nat.add(a1, b1), c1), Nat.add(Nat.add(a2, b2), c2),
          Nat.add(Nat.add(a3, b3), c3), Nat.add(Nat.add(a4, b4), c4), _} : FN.Totals}
      {==}

def Laws.fn_total_empty():
  {==}

def Laws.fn_total_append(xs, ys):
  match xs:
    case Nil{}:
      fn.zero_plus(FN.total(ys))
    case Con{f, t}:
      %Equal.sym(FN.Totals, FN.plus(FN.plus(FN.of(f), FN.total(t)), FN.total(ys)),
        FN.plus(FN.of(f), FN.plus(FN.total(t), FN.total(ys))),
        fn.plus_assoc(FN.of(f), FN.total(t), FN.total(ys))) :
        {FN.plus(FN.of(f), FN.total(List.append(&2, FN.Funnel, t, ys))) == _ : FN.Totals}
      %Laws.fn_total_append(t, ys) :
        {FN.plus(FN.of(f), FN.total(List.append(&2, FN.Funnel, t, ys)))
          == FN.plus(FN.of(f), _) : FN.Totals}
      {==}

def Laws.fn_zero_match_tagged(rest):
  {==}

def Laws.fn_funnel_not_zero_match(rest):
  {==}

# ---------------------------------------------------------------------------
# Pagination
# ---------------------------------------------------------------------------

def Laws.pg_page_min(raw):
  L.le_one_succ(Nat.min(99999n, PG.pred_or(PG.parse(raw), 0n)))

def Laws.pg_page_max(raw):
  L.min_le_l(99999n, PG.pred_or(PG.parse(raw), 0n))

def Laws.pg_limit_min(raw):
  L.le_one_succ(Nat.min(99n, PG.pred_or(PG.parse(raw), 26n)))

def Laws.pg_limit_max(raw):
  L.min_le_l(99n, PG.pred_or(PG.parse(raw), 26n))

def Laws.pg_defaults():
  {==}

def Laws.pg_first_page_offset(limit):
  {==}

# ---------------------------------------------------------------------------
# Job slugs
# ---------------------------------------------------------------------------

law js.len_append:
  for +a: String
  for +b: String
  {Nat.add(String.length(a), String.length(b)) == String.length(a ++ b) : Nat}

def js.len_append(a, b):
  match a:
    case SNil{}:
      {==}
    case SCon{h, t}:
      %js.len_append(t, b) :
        {1n+Nat.add(String.length(t), String.length(b)) == 1n+_ : Nat}
      {==}

law js.drop_append:
  for +a: String
  for +b: String
  {b == String.drop(a ++ b, String.length(a)) : String}

def js.drop_append(a, b):
  match a b:
    case SNil{} SNil{}:
      {==}
    case SNil{} SCon{h, t}:
      {==}
    case SCon{h, t} _:
      js.drop_append(t, b)

law js.step:
  for +x: JS.Hex
  for +r: String
  {JS.hex_digits.if(JS.is_hex(JS.hex_char(x)), Char.to_lower(JS.hex_char(x)), Some{r})
    == Some{SCon{JS.hex_char(x), r}} : Maybe<&2, String>}

def js.step(x, r):
  match x:
    case JS.X0{}:
      {==}
    case JS.X1{}:
      {==}
    case JS.X2{}:
      {==}
    case JS.X3{}:
      {==}
    case JS.X4{}:
      {==}
    case JS.X5{}:
      {==}
    case JS.X6{}:
      {==}
    case JS.X7{}:
      {==}
    case JS.X8{}:
      {==}
    case JS.X9{}:
      {==}
    case JS.Xa{}:
      {==}
    case JS.Xb{}:
      {==}
    case JS.Xc{}:
      {==}
    case JS.Xd{}:
      {==}
    case JS.Xe{}:
      {==}
    case JS.Xf{}:
      {==}

law js.digits_hex:
  for +xs: List<&2, JS.Hex>
  {JS.hex_digits(JS.hex_str(xs)) == Some{JS.hex_str(xs)} : Maybe<&2, String>}

def js.digits_hex(xs):
  match xs:
    case Nil{}:
      {==}
    case Con{x, t}:
      %Equal.sym(Maybe<&2, String>, JS.hex_digits(JS.hex_str(t)), Some{JS.hex_str(t)},
        js.digits_hex(t)) :
        {JS.hex_digits.if(JS.is_hex(JS.hex_char(x)), Char.to_lower(JS.hex_char(x)), _)
          == Some{SCon{JS.hex_char(x), JS.hex_str(t)}} : Maybe<&2, String>}
      js.step(x, JS.hex_str(t))

def Laws.js_roundtrip(base, id):
  match id:
    case JS.Hex8{a, b, c, d, e, f, g, h}:
      +hs = {[a, b, c, d, e, f, g, h] : List<&2, JS.Hex>}
      +tail = {SCon{'-', JS.hex_str(hs)} : String}
      %js.len_append(base, tail) :
        {JS.parse.tail(String.drop(base ++ tail, Nat.sub(_, 9n)))
          == Some{JS.hex_str(hs)} : Maybe<&2, String>}
      %Equal.sym(Nat, Nat.sub(Nat.add(String.length(base), 9n), 9n),
        String.length(base), L.add_sub(String.length(base), 9n)) :
        {JS.parse.tail(String.drop(base ++ tail, _))
          == Some{JS.hex_str(hs)} : Maybe<&2, String>}
      %js.drop_append(base, tail) :
        {JS.parse.tail(_) == Some{JS.hex_str(hs)} : Maybe<&2, String>}
      js.digits_hex(hs)
