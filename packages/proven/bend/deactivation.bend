import Base

# Which of a company's existing jobs an ingest run may retire.

# What one run learned about a company's board.
type Run is Data:
  Failed{}
  Empty{}
  Listed{}

# Where one existing job stands on a Listed board.
type Seen is Data:
  Gone{}
  Rejected{}
  Unverified{}
  Matched{}

# Empty runs in a row before every job of the company is retired.
def retire_after_empty() -> Nat:
  3n

# Run: Failed = fetch failed or refused; Empty = fetched, no postings;
# Listed = fetched, at least one posting.
# Seen: Gone = not on the board; Rejected = on the board with details, fails the
# classifier; Unverified = on the board, details missing; Matched = passes.
# prior_empty: Empty runs in a row before this one.
def retire(run: Run, prior_empty: Nat, seen: Seen) -> Bool:
  match run:
    case Failed{}:
      False{}
    case Empty{}:
      Nat.is_le(retire_after_empty(), 1n+prior_empty)
    case Listed{}:
      match seen:
        case Gone{}:
          True{}
        case Rejected{}:
          True{}
        case Unverified{}:
          False{}
        case Matched{}:
          False{}

# The Empty-run count after this run.
def next_empty(run: Run, prior_empty: Nat) -> Nat:
  match run:
    case Failed{}:
      prior_empty
    case Empty{}:
      1n+prior_empty
    case Listed{}:
      0n
