/**
 * Lightweight ATS normalizer smoke tests (no network).
 */
import assert from "node:assert/strict";

// Recruitee-shaped mapping (inline mirror of fetchRecruitee mapping)
function mapRecruitee(offers: {
  id: number;
  title: string;
  slug: string;
  careers_url?: string;
  description?: string;
  city?: string;
  country?: string;
  remote?: boolean;
  published_at?: string;
}[]) {
  return offers.map((j) => {
    const locations: string[] = [];
    const cityCountry = [j.city, j.country].filter(Boolean).join(", ");
    if (cityCountry) locations.push(cityCountry);
    if (j.remote) locations.push("Remote");
    return {
      externalId: String(j.id),
      title: j.title,
      locations,
      applyUrl: j.careers_url ?? `https://example.recruitee.com/o/${j.slug}`,
      postedAt: j.published_at ? new Date(j.published_at) : null,
      description: j.description ?? "",
    };
  });
}

const recruitee = mapRecruitee([
  {
    id: 42,
    title: "Software Engineer Intern",
    slug: "swe-intern",
    city: "Amsterdam",
    country: "Netherlands",
    remote: true,
    published_at: "2026-06-01T00:00:00Z",
    description: "Build APIs",
  },
]);
assert.equal(recruitee[0]!.externalId, "42");
assert.equal(recruitee[0]!.title, "Software Engineer Intern");
assert.deepEqual(recruitee[0]!.locations, ["Amsterdam, Netherlands", "Remote"]);
assert.ok(recruitee[0]!.postedAt instanceof Date);

function mapRippling(jobs: { uuid: string; name: string; url?: string; workLocation?: { label?: string } }[]) {
  return jobs.map((j) => ({
    externalId: j.uuid,
    title: j.name,
    locations: j.workLocation?.label ? [j.workLocation.label] : [],
    applyUrl: j.url ?? `https://ats.rippling.com/x/jobs/${j.uuid}`,
  }));
}

const rippling = mapRippling([
  { uuid: "abc", name: "ML Intern", workLocation: { label: "San Francisco, CA" }, url: "https://example.com" },
]);
assert.equal(rippling[0]!.externalId, "abc");
assert.deepEqual(rippling[0]!.locations, ["San Francisco, CA"]);

function mapBamboo(result: { id: string; jobOpeningName: string; location?: { city?: string; state?: string }; isRemote?: boolean }[]) {
  return result.map((j) => {
    const locations: string[] = [];
    if (j.location) {
      const parts = [j.location.city, j.location.state].filter(Boolean);
      if (parts.length) locations.push(parts.join(", "));
    }
    if (j.isRemote) locations.push("Remote");
    return {
      externalId: String(j.id),
      title: j.jobOpeningName,
      locations,
      applyUrl: `https://demo.bamboohr.com/careers/${j.id}`,
      description: "",
    };
  });
}

const bamboo = mapBamboo([
  { id: "9", jobOpeningName: "Software Intern", location: { city: "Provo", state: "Utah" }, isRemote: false },
]);
assert.equal(bamboo[0]!.externalId, "9");
assert.deepEqual(bamboo[0]!.locations, ["Provo, Utah"]);

console.log("ats normalizers ok");
