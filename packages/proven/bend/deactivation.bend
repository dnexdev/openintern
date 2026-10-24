import Base

# Which of a company's existing jobs an ingest run may deactivate.

type Policy is Data:
  Missing{}
  All{}
  Keep{}

def policy(fetch_ok: Bool, fetched: Nat, tech: Nat) -> Policy:
  match fetch_ok:
    case False{}:
      Keep{}
    case True{}:
      match fetched tech:
        case _ 1n+t:
          Missing{}
        case 0n 0n:
          All{}
        case 1n+f 0n:
          Keep{}
