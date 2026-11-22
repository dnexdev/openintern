import Base

# Apply-button decisions for a role family. `postings` is the number of
# postings in the family; `locations` is the number of non-blank locations
# on its first posting.

# True when the user must pick a row: never send Apply to an arbitrary posting.
def needs_picker(postings: Nat, locations: Nat) -> Bool:
  match postings:
    case 0n:
      False{}
    case 1n+p:
      match p:
        case 0n:
          Nat.is_le(2n, locations)
        case 1n+q:
          True{}

# A lone posting's applied title names its locations when it has several.
def qualify_single_title(locations: Nat) -> Bool:
  Nat.is_le(2n, locations)

# Header copy: "N openings" when locations repeat, else "N locations".
def says_openings(postings: Nat, unique_locations: Nat) -> Bool:
  Nat.is_lt(unique_locations, postings)
