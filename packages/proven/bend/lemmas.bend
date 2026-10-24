import Base

# Generic facts about Bool and Nat that PROOF.bend leans on. Each is a law
# proven right below it, so this file checks on its own.

def TrueDisc(b: Bool) -> Type:
  match b:
    case True{}:
      Empty
    case False{}:
      Unit

law ft_empty:
  for e: {False{} == True{} : Bool}
  Empty

def ft_empty(e):
  %e : TrueDisc(_)
  Unit{}

def empty_elim(-P: Type, x: Empty) -> P:
  match x:

# Ex falso: a False == True hypothesis proves anything.
law ft:
  for -P: Type
  for e: {False{} == True{} : Bool}
  P

def ft(P, e):
  empty_elim(P, ft_empty(e))

law not_or:
  for +a: Bool
  for +b: Bool
  {Bool.not(Bool.or(a, b)) == Bool.and(Bool.not(a), Bool.not(b)) : Bool}

def not_or(a, b):
  match a:
    case True{}:
      {==}
    case False{}:
      {==}

law not_not:
  for +a: Bool
  {Bool.not(Bool.not(a)) == a : Bool}

def not_not(a):
  match a:
    case True{}:
      {==}
    case False{}:
      {==}

law le_refl:
  for +a: Nat
  {Nat.is_le(a, a) == True{} : Bool}

def le_refl(a):
  match a:
    case 0n:
      {==}
    case 1n+p:
      le_refl(p)

law le_add:
  for +a: Nat
  for +k: Nat
  {Nat.is_le(a, Nat.add(a, k)) == True{} : Bool}

def le_add(a, k):
  match a k:
    case 0n 0n:
      {==}
    case 0n 1n+j:
      {==}
    case 1n+p _:
      le_add(p, k)

law lt_add_succ:
  for +a: Nat
  for +k: Nat
  {Nat.is_le(1n+Nat.add(a, k), a) == False{} : Bool}

def lt_add_succ(a, k):
  match a:
    case 0n:
      {==}
    case 1n+p:
      lt_add_succ(p, k)

law lt_not_le:
  for +a: Nat
  for +b: Nat
  {Nat.is_lt(a, b) == Bool.not(Nat.is_le(b, a)) : Bool}

def lt_not_le(a, b):
  match a b:
    case 0n 0n:
      {==}
    case 0n 1n+q:
      {==}
    case 1n+p 0n:
      {==}
    case 1n+p 1n+q:
      lt_not_le(p, q)

law add_zero:
  for +a: Nat
  {Nat.add(a, 0n) == a : Nat}

def add_zero(a):
  match a:
    case 0n:
      {==}
    case 1n+p:
      %add_zero(p) : {1n+Nat.add(p, 0n) == 1n+_ : Nat}
      {==}

law add_succ:
  for +a: Nat
  for +b: Nat
  {Nat.add(a, 1n+b) == 1n+Nat.add(a, b) : Nat}

def add_succ(a, b):
  match a:
    case 0n:
      {==}
    case 1n+p:
      %add_succ(p, b) : {1n+Nat.add(p, 1n+b) == 1n+_ : Nat}
      {==}

law add_assoc:
  for +a: Nat
  for +b: Nat
  for +c: Nat
  {Nat.add(Nat.add(a, b), c) == Nat.add(a, Nat.add(b, c)) : Nat}

def add_assoc(a, b, c):
  match a:
    case 0n:
      {==}
    case 1n+p:
      %add_assoc(p, b, c) : {1n+Nat.add(Nat.add(p, b), c) == 1n+_ : Nat}
      {==}

law sub_zero:
  for +a: Nat
  {Nat.sub(a, 0n) == a : Nat}

def sub_zero(a):
  match a:
    case 0n:
      {==}
    case 1n+p:
      {==}

law add_sub:
  for +a: Nat
  for +b: Nat
  {Nat.sub(Nat.add(a, b), b) == a : Nat}

def add_sub(a, b):
  match b:
    case 0n:
      Equal.trans(Nat, Nat.sub(Nat.add(a, 0n), 0n), Nat.add(a, 0n), a,
        sub_zero(Nat.add(a, 0n)), add_zero(a))
    case 1n+q:
      %Equal.sym(Nat, Nat.add(a, 1n+q), 1n+Nat.add(a, q), add_succ(a, q)) :
        {Nat.sub(_, 1n+q) == a : Nat}
      add_sub(a, q)

law and_l:
  for +a: Bool
  for +b: Bool
  for e: {Bool.and(a, b) == True{} : Bool}
  {a == True{} : Bool}

def and_l(a, b, e):
  match a:
    case True{}:
      {==}
    case False{}:
      ft({False{} == True{} : Bool}, e)

law and_r:
  for +a: Bool
  for +b: Bool
  for e: {Bool.and(a, b) == True{} : Bool}
  {b == True{} : Bool}

def and_r(a, b, e):
  match a:
    case True{}:
      e
    case False{}:
      ft({b == True{} : Bool}, e)

law and_intro:
  for +a: Bool
  for +b: Bool
  for ea: {a == True{} : Bool}
  for eb: {b == True{} : Bool}
  {Bool.and(a, b) == True{} : Bool}

def and_intro(a, b, ea, eb):
  %Equal.sym(Bool, a, True{}, ea) : {Bool.and(_, b) == True{} : Bool}
  eb

law lt_mono:
  for +x: Nat
  for +n: Nat
  for +k: Nat
  for e: {Nat.is_lt(x, n) == True{} : Bool}
  {Nat.is_lt(x, Nat.add(n, k)) == True{} : Bool}

def lt_mono(x, n, k, e):
  match x n:
    case 0n 0n:
      ft({Nat.is_lt(0n, Nat.add(0n, k)) == True{} : Bool}, e)
    case 0n 1n+q:
      {==}
    case 1n+p 0n:
      ft({Nat.is_lt(1n+p, Nat.add(0n, k)) == True{} : Bool}, e)
    case 1n+p 1n+q:
      lt_mono(p, q, k, e)

law lt_irrefl:
  for +a: Nat
  {Nat.is_lt(a, a) == False{} : Bool}

def lt_irrefl(a):
  match a:
    case 0n:
      {==}
    case 1n+p:
      lt_irrefl(p)

law lt_succ_add:
  for +a: Nat
  for +k: Nat
  {Nat.is_lt(a, 1n+Nat.add(a, k)) == True{} : Bool}

def lt_succ_add(a, k):
  match a:
    case 0n:
      {==}
    case 1n+p:
      lt_succ_add(p, k)

law le_one_succ:
  for +m: Nat
  {Nat.is_le(1n, 1n+m) == True{} : Bool}

def le_one_succ(m):
  match m:
    case 0n:
      {==}
    case 1n+p:
      {==}

law min_le_l:
  for +a: Nat
  for +b: Nat
  {Nat.is_le(Nat.min(a, b), a) == True{} : Bool}

def min_le_l(a, b):
  match a b:
    case 0n _:
      {==}
    case 1n+p 0n:
      {==}
    case 1n+p 1n+q:
      min_le_l(p, q)
