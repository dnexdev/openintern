export type Funnel = {
  fetched: number | null;
  title: number | null;
  tech: number | null;
  upserted: number | null;
};

export type FunnelTotals = {
  fetched: number;
  title: number;
  tech: number;
  upserted: number;
};

export function parseFunnel(error: string | null): Funnel {
  if (!error) return { fetched: null, title: null, tech: null, upserted: null };
  const match = error.match(
    /funnel fetched=(\d+) title=(\d+) tech=(\d+) upserted=(\d+)/,
  );
  if (!match) return { fetched: null, title: null, tech: null, upserted: null };
  return {
    fetched: Number(match[1]),
    title: Number(match[2]),
    tech: Number(match[3]),
    upserted: Number(match[4]),
  };
}

export function sumFunnels(funnels: Funnel[]): FunnelTotals {
  return funnels.reduce<FunnelTotals>(
    (total, funnel) => ({
      fetched: total.fetched + (funnel.fetched ?? 0),
      title: total.title + (funnel.title ?? 0),
      tech: total.tech + (funnel.tech ?? 0),
      upserted: total.upserted + (funnel.upserted ?? 0),
    }),
    { fetched: 0, title: 0, tech: 0, upserted: 0 },
  );
}
