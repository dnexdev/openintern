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

type WorkableJob = {
  shortcode?: string;
  id?: string;
  title: string;
  url?: string;
  application_url?: string;
  shortlink?: string;
  location?: { city?: string; region?: string; country?: string; telecommuting?: boolean };
  locations?: { city?: string; region?: string; country?: string }[];
  telecommuting?: boolean;
  published_on?: string;
  created_at?: string;
  description?: string;
  full_description?: string;
};

type WorkableResponse = { name?: string; jobs?: WorkableJob[] };

function workableLocation(loc?: {
  city?: string;
  region?: string;
  country?: string;
}): string | null {
  if (!loc) return null;
  const parts = [loc.city, loc.region, loc.country].filter(Boolean);
  return parts.length > 0 ? parts.join(", ") : null;
}

export async function fetchWorkable(boardToken: string): Promise<NormalizedJob[]> {
  const data = await fetchJson<WorkableResponse>(
    `https://www.workable.com/api/accounts/${encodeURIComponent(boardToken)}?details=true`,
  );
  return (data.jobs ?? []).map((j) => {
    const locations: string[] = [];
    const primary = workableLocation(j.location);
    if (primary) locations.push(primary);
    for (const extra of j.locations ?? []) {
      const s = workableLocation(extra);
      if (s && !locations.includes(s)) locations.push(s);
    }
    if ((j.telecommuting || j.location?.telecommuting) && !locations.some((l) => /remote/i.test(l))) {
      locations.push("Remote");
    }
    const description = j.full_description ?? j.description ?? "";
    const posted = j.published_on ?? j.created_at;
    return {
      externalId: j.shortcode ?? j.id ?? j.url ?? j.title,
      title: j.title,
      locations,
      applyUrl: j.application_url ?? j.url ?? j.shortlink ?? "",
      excerpt: null,
      postedAt: posted ? new Date(posted) : null,
      description,
    };
  });
}

type SmartRecruitersPosting = {
  id: string;
  uuid?: string;
  name: string;
  releasedDate?: string;
  location?: { city?: string; region?: string; country?: string; remote?: boolean };
  company?: { identifier?: string };
  jobAd?: { sections?: Record<string, { title?: string; text?: string }> };
  applyUrl?: string;
  postingUrl?: string;
  ref?: string;
};

type SmartRecruitersResponse = { content?: SmartRecruitersPosting[]; totalFound?: number };

export async function fetchSmartRecruiters(boardToken: string): Promise<NormalizedJob[]> {
  const limit = 100;
  const all: SmartRecruitersPosting[] = [];
  for (let offset = 0; ; offset += limit) {
    const data = await fetchJson<SmartRecruitersResponse>(
      `https://api.smartrecruiters.com/v1/companies/${encodeURIComponent(boardToken)}/postings?limit=${limit}&offset=${offset}`,
    );
    const batch = data.content ?? [];
    all.push(...batch);
    if (batch.length < limit) break;
  }
  return all.map((j) => {
    const locations: string[] = [];
    const parts = [j.location?.city, j.location?.region, j.location?.country].filter(Boolean);
    if (parts.length > 0) locations.push(parts.join(", "));
    if (j.location?.remote && !locations.some((l) => /remote/i.test(l))) {
      locations.push("Remote");
    }
    const description = Object.values(j.jobAd?.sections ?? {})
      .map((s) => s?.text ?? "")
      .join(" ");
    return {
      externalId: j.uuid ?? j.id,
      title: j.name,
      locations,
      applyUrl:
        j.applyUrl ??
        j.postingUrl ??
        `https://jobs.smartrecruiters.com/${encodeURIComponent(boardToken)}/${j.id}`,
      excerpt: null,
      postedAt: j.releasedDate ? new Date(j.releasedDate) : null,
      description,
    };
  });
}

export async function fetchJobsForAts(
  ats: "greenhouse" | "lever" | "ashby" | "workable" | "smartrecruiters",
  boardToken: string,
): Promise<NormalizedJob[]> {
  switch (ats) {
    case "greenhouse":
      return fetchGreenhouse(boardToken);
    case "lever":
      return fetchLever(boardToken);
    case "ashby":
      return fetchAshby(boardToken);
    case "workable":
      return fetchWorkable(boardToken);
    case "smartrecruiters":
      return fetchSmartRecruiters(boardToken);
    default:
      throw new Error(`Unsupported ATS: ${ats}`);
  }
}
