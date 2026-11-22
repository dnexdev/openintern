import Base

# Job URL segments: "<slugified title>-<first 8 hex chars of the uuid>".
# Slugifying the title needs Unicode classes and stays in TS; the id part,
# which routing depends on, lives here.

type Hex is Data:
  X0{}
  X1{}
  X2{}
  X3{}
  X4{}
  X5{}
  X6{}
  X7{}
  X8{}
  X9{}
  Xa{}
  Xb{}
  Xc{}
  Xd{}
  Xe{}
  Xf{}

# The 8 hex digits a job id prefix is made of.
type Hex8 is Data:
  Hex8{a: Hex, b: Hex, c: Hex, d: Hex, e: Hex, f: Hex, g: Hex, h: Hex}

def hex_char(x: Hex) -> Char:
  match x:
    case X0{}:
      '0'
    case X1{}:
      '1'
    case X2{}:
      '2'
    case X3{}:
      '3'
    case X4{}:
      '4'
    case X5{}:
      '5'
    case X6{}:
      '6'
    case X7{}:
      '7'
    case X8{}:
      '8'
    case X9{}:
      '9'
    case Xa{}:
      'a'
    case Xb{}:
      'b'
    case Xc{}:
      'c'
    case Xd{}:
      'd'
    case Xe{}:
      'e'
    case Xf{}:
      'f'

def hex_str(xs: List<&2, Hex>) -> String:
  match xs:
    case Nil{}:
      SNil{}
    case Con{x, t}:
      SCon{hex_char(x), hex_str(t)}

def hex8_str(x: Hex8) -> String:
  match x:
    case Hex8{a, b, c, d, e, f, g, h}:
      hex_str([a, b, c, d, e, f, g, h])

def le_f(c: Char) -> Bool:
  match c:
    case Chr{x}:
      U32.is_le(x, 102)

def is_hex(+c: Char) -> Bool:
  +l = Char.to_lower(c)
  Bool.or(Char.is_digit(c),
    Bool.and(Char.is_lower(l), le_f(l)))

def hex_digits.if(ok: Bool, c: Char, rest: Maybe<&2, String>) -> Maybe<&2, String>:
  match ok rest:
    case False{} _:
      None{}
    case True{} None{}:
      None{}
    case True{} Some{r}:
      Some{SCon{c, r}}

# Lowercased hex digits, or None if any char is not a hex digit.
def hex_digits(s: String) -> Maybe<&2, String>:
  match s:
    case SNil{}:
      Some{SNil{}}
    case SCon{+c, t}:
      hex_digits.if(is_hex(c), Char.to_lower(c), hex_digits(t))

def strip_dashes.if(dash: Bool, c: Char, rest: String) -> String:
  match dash:
    case True{}:
      rest
    case False{}:
      SCon{c, rest}

def strip_dashes(s: String) -> String:
  match s:
    case SNil{}:
      SNil{}
    case SCon{+c, t}:
      strip_dashes.if(Char.is_eq(c, '-'), c, strip_dashes(t))

# First 8 chars of the id with dashes removed.
def suffix(id: String) -> String:
  String.take(strip_dashes(id), 8n)

def segment(base: String, id: String) -> String:
  base ++ "-" ++ suffix(id)

def parse.dash(dash: Bool, len8: Bool, digits: Maybe<&2, String>) -> Maybe<&2, String>:
  match dash len8:
    case True{} True{}:
      digits
    case _ _:
      None{}

def parse.tail(s: String) -> Maybe<&2, String>:
  match s:
    case SNil{}:
      None{}
    case SCon{c, +r}:
      parse.dash(Char.is_eq(c, '-'), Nat.is_eq(String.length(r), 8n), hex_digits(r))

# The id prefix at the end of a job slug: "-" then 8 hex digits, any case.
def parse(+slug: String) -> Maybe<&2, String>:
  parse.tail(String.drop(slug, Nat.sub(String.length(slug), 9n)))
