import {
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
    fetched: Number(match[1]),
    title: Number(match[2]),
    tech: Number(match[3]),
    upserted: Number(match[4]),
    descriptionMissing: match[5] != null ? Number(match[5]) : null,
  };
}

export function sumFunnels(funnels: Funnel[]): FunnelTotals {
  return provenSumFunnels(funnels);
}

export function isZeroMatchRun(error: string | null): boolean {
  return provenIsZeroMatchRun(error);
}
