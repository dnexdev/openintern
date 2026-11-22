import {
  NAT_LIMIT,
  isZeroMatchRun as provenIsZeroMatchRun,
  sumFunnels as provenSumFunnels,
} from "@openintern/proven";

export type Funnel = {
  fetched: number | null;
  title: number | null;
  tech: number | null;
  upserted: number | null;
  descriptionMissing: number | null;
};

export type FunnelTotals = {
  fetched: number;
  title: number;
  tech: number;
  upserted: number;
  descriptionMissing: number;
};

/** A funnel count, or null when missing or too large to be real. */
function count(digits: string | undefined): number | null {
  if (digits == null) return null;
  const n = Number(digits);
  return n < NAT_LIMIT ? n : null;
}

export function parseFunnel(error: string | null): Funnel {
  if (!error) {
    return {
      fetched: null,
      title: null,
      tech: null,
      upserted: null,
      descriptionMissing: null,
    };
  }
  const match = error.match(
    /funnel fetched=(\d+) title=(\d+) tech=(\d+) upserted=(\d+)(?: description_missing=(\d+))?/,
  );
  if (!match) {
    return {
      fetched: null,
      title: null,
      tech: null,
      upserted: null,
      descriptionMissing: null,
    };
  }
  return {
    fetched: count(match[1]),
    title: count(match[2]),
    tech: count(match[3]),
    upserted: count(match[4]),
    descriptionMissing: count(match[5]),
  };
}

export function sumFunnels(funnels: Funnel[]): FunnelTotals {
  return provenSumFunnels(funnels);
}

export function isZeroMatchRun(error: string | null): boolean {
  return provenIsZeroMatchRun(error);
}
