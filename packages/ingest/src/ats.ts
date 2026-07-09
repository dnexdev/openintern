export type NormalizedJob = {
  externalId: string;
  title: string;
  locations: string[];
  applyUrl: string;
  excerpt: string | null;
  postedAt: Date | null;
  description: string;
};

async function fetchJson<T>(url: string): Promise<T> {
  const res = await fetch(url, {
    headers: {
      Accept: "application/json",
      "User-Agent": "OpenIntern/0.1 (+https://github.com/dnexdev/openintern)",
    },
  });
  if (!res.ok) {
    throw new Error(`HTTP ${res.status} for ${url}`);
  }
  return (await res.json()) as T;
}

type GreenhouseJob = {
  id: number;
  title: string;
  absolute_url: string;
  updated_at?: string;
  location?: { name?: string };
  content?: string;
};

type GreenhouseResponse = { jobs: GreenhouseJob[] };

export async function fetchGreenhouse(boardToken: string): Promise<NormalizedJob[]> {
  const data = await fetchJson<GreenhouseResponse>(
    `https://boards-api.greenhouse.io/v1/boards/${encodeURIComponent(boardToken)}/jobs?content=true`,
  );
  return (data.jobs ?? []).map((j) => ({
    externalId: String(j.id),
    title: j.title,
    locations: j.location?.name ? [j.location.name] : [],
    applyUrl: j.absolute_url,
    excerpt: null,
    postedAt: j.updated_at ? new Date(j.updated_at) : null,
    description: j.content ?? "",
  }));
}

type LeverJob = {
  id: string;
  text: string;
  hostedUrl: string;
  createdAt?: number;
  categories?: { location?: string; commitment?: string; team?: string };
  descriptionPlain?: string;
  description?: string;
};

export async function fetchLever(boardToken: string): Promise<NormalizedJob[]> {
  const data = await fetchJson<LeverJob[]>(
    `https://api.lever.co/v0/postings/${encodeURIComponent(boardToken)}?mode=json`,
  );
  return (data ?? []).map((j) => ({
    externalId: j.id,
    title: j.text,
    locations: j.categories?.location ? [j.categories.location] : [],
    applyUrl: j.hostedUrl,
    excerpt: j.descriptionPlain?.slice(0, 400) ?? null,
    postedAt: j.createdAt ? new Date(j.createdAt) : null,
    description: j.descriptionPlain ?? j.description ?? "",
  }));
}

type AshbyJob = {
  id: string;
  title: string;
  jobUrl: string;
  location?: string;
  isRemote?: boolean;
  publishedAt?: string;
  descriptionPlain?: string;
  descriptionHtml?: string;
};

type AshbyResponse = { jobs: AshbyJob[] };

export async function fetchAshby(boardToken: string): Promise<NormalizedJob[]> {
  const data = await fetchJson<AshbyResponse>(
    `https://api.ashbyhq.com/posting-api/job-board/${encodeURIComponent(boardToken)}?includeCompensation=true`,
  );
  return (data.jobs ?? []).map((j) => {
    const locations: string[] = [];
    if (j.location) locations.push(j.location);
    if (j.isRemote && !locations.some((l) => /remote/i.test(l))) {
      locations.push("Remote");
    }
    return {
      externalId: j.id,
      title: j.title,
      locations,
      applyUrl: j.jobUrl,
      excerpt: j.descriptionPlain?.slice(0, 400) ?? null,
      postedAt: j.publishedAt ? new Date(j.publishedAt) : null,
      description: j.descriptionPlain ?? j.descriptionHtml ?? "",
    };
  });
}

export async function fetchJobsForAts(
  ats: "greenhouse" | "lever" | "ashby",
  boardToken: string,
): Promise<NormalizedJob[]> {
  switch (ats) {
    case "greenhouse":
      return fetchGreenhouse(boardToken);
    case "lever":
      return fetchLever(boardToken);
    case "ashby":
      return fetchAshby(boardToken);
    default:
      throw new Error(`Unsupported ATS: ${ats}`);
  }
}
