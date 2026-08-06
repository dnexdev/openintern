/**
 * Proprietary careers adapter unit tests (no network).
 */
import assert from "node:assert/strict";
import { parseCitadelListingHtml } from "./citadel.js";
import { parseTeslaCareersState } from "./tesla.js";
import { mapBytedancePosts, TIKTOK_PORTAL, BYTEDANCE_PORTAL } from "./bytedance.js";
import { mapPhenomJobs } from "./phenom.js";
import { mapEightfoldPositions, parseEightfoldToken } from "./eightfold.js";
import { parseAvatureSearchHtml } from "./avature.js";
import { parseIcimsSearchHtml } from "./icims.js";
import { parseDeshawNextData } from "./deshaw.js";
import { parseShopifyCareersHtml } from "./shopify.js";
import { parseGoogleCareersHtml } from "./google.js";

const citadelHtml = `
<div class="career-listing__container">
<a class="careers-listing-card js-career-card js-apply-now" href="https://www.citadel.com/careers/details/software-engineer-intern-us/" data-position="Software Engineer – Intern (US)">
  <div class="careers-listing-card__content">
    <div class="careers-listing-card__title">
      <h2>Software Engineer &#8211; Intern (US)</h2>
    </div>
    <span class="careers-listing-card__location">Greenwich, Houston, Miami, New York</span>
  </div>
</a>
<a class="careers-listing-card js-career-card" href="/careers/details/software-engineer/">
  <div class="careers-listing-card__title"><h2>Software Engineer</h2></div>
  <span class="careers-listing-card__location">New York</span>
</a>
</div>
`;

const citadelJobs = parseCitadelListingHtml(citadelHtml, "https://www.citadel.com");
assert.equal(citadelJobs.length, 2);
assert.equal(citadelJobs[0]!.externalId, "software-engineer-intern-us");
assert.equal(citadelJobs[0]!.title, "Software Engineer – Intern (US)");
assert.deepEqual(citadelJobs[0]!.locations, ["Greenwich", "Houston", "Miami", "New York"]);
assert.equal(
  citadelJobs[0]!.applyUrl,
  "https://www.citadel.com/careers/details/software-engineer-intern-us/",
);

const teslaState = {
  sites: { "1": "Fremont, California", "2": "Austin, Texas" },
  listings: [
    { id: 233415, t: "Internship, Software Engineer", sid: "1", d: "Build cars" },
    { id: "99", title: "Staff Engineer", l: "Remote" },
  ],
};
const teslaJobs = parseTeslaCareersState(teslaState);
assert.equal(teslaJobs.length, 2);
assert.equal(teslaJobs[0]!.externalId, "233415");
assert.equal(teslaJobs[0]!.title, "Internship, Software Engineer");
assert.deepEqual(teslaJobs[0]!.locations, ["Fremont, California"]);
assert.equal(teslaJobs[0]!.applyUrl, "https://www.tesla.com/careers/search/job/233415");

const mapped = mapBytedancePosts(
  [
    {
      id: "7657410043649804597",
      title: "Platform Strategy Project Intern",
      description: "Shop intern role",
      city_info: {
        en_name: "Singapore",
        parent: { en_name: "Singapore", parent: { en_name: "Singapore" } },
      },
      recruit_type: { id: "202", en_name: "Intern" },
    },
  ],
  TIKTOK_PORTAL,
);
assert.equal(mapped[0]!.externalId, "7657410043649804597");
assert.equal(
  mapped[0]!.applyUrl,
  "https://lifeattiktok.com/search/7657410043649804597",
);
assert.ok(mapped[0]!.locations[0]?.includes("Singapore"));

const bd = mapBytedancePosts(
  [{ id: "1", title: "SWE Intern", description: "x" }],
  BYTEDANCE_PORTAL,
);
assert.equal(bd[0]!.applyUrl, "https://joinbytedance.com/search/1");

const phenomJobs = mapPhenomJobs(
  [
    {
      data: {
        req_id: "77690",
        slug: "77690",
        title: "FPGA Design Intern",
        description: "<p>Build chips</p>",
        full_location: "Austin, TX",
        meta_data: { canonical_url: "https://careers.amd.com/jobs/77690?lang=en-us" },
        posted_date: "2026-08-01T00:00:00Z",
      },
    },
  ],
  "https://careers.amd.com",
);
assert.equal(phenomJobs[0]!.externalId, "77690");
assert.equal(phenomJobs[0]!.title, "FPGA Design Intern");
assert.equal(phenomJobs[0]!.applyUrl, "https://careers.amd.com/jobs/77690?lang=en-us");

const efBoard = parseEightfoldToken("microsoft|ai");
assert.equal(efBoard.domain, "microsoft.com");
assert.ok(efBoard.titleFilter);
const efJobs = mapEightfoldPositions(
  [
    {
      id: 1970393556951950,
      name: "Software Engineer: AI/ML & LLM Intern Opportunities",
      locations: ["United States, Washington, Redmond"],
      positionUrl: "/careers/job/1970393556951950",
      postedTs: 1785740400,
      department: "Software Engineering",
    },
    {
      id: 99,
      name: "Account Management INTERN",
      locations: ["Germany"],
      positionUrl: "/careers/job/99",
    },
  ],
  efBoard,
);
assert.equal(efJobs.length, 1);
assert.equal(efJobs[0]!.externalId, "1970393556951950");
assert.match(efJobs[0]!.applyUrl, /1970393556951950/);

const avatureHtml2 = `
<a class="link" href="https://bloomberg.avature.net/careers/JobDetail/Software-Engineer-Intern/21267">Software Engineer Intern</a>
<div>JobDetail/Software-Engineer-Intern/21267</div>
<span class="list-item-location">New York, NY</span>
`;
const avJobs = parseAvatureSearchHtml(avatureHtml2, "https://bloomberg.avature.net/careers/SearchJobs/");
assert.equal(avJobs.length, 1);
assert.equal(avJobs[0]!.externalId, "21267");
assert.equal(avJobs[0]!.title, "Software Engineer Intern");

const icimsHtml = `
<a href="https://careers-sig.icims.com/jobs/11099/analytics-internship%3a-fall-2026/job?in_iframe=1"></a>
<h3>Analytics Internship: Fall 2026</h3>
`;
const icJobs = parseIcimsSearchHtml(icimsHtml, "https://careers-sig.icims.com/jobs/search");
assert.equal(icJobs.length, 1);
assert.equal(icJobs[0]!.externalId, "11099");
assert.match(icJobs[0]!.title, /Analytics Internship/i);

const deshawJobs = parseDeshawNextData({
  props: {
    pageProps: {
      internships: [
        {
          office: [{ name: "New York" }],
          data: {
            id: 5894,
            displayName: "Software Developer Intern (New York) – Summer 2027",
            jobUrl: "Software-Developer-Intern-New-York-Summer-2027-5894",
            jobDescription: "<p>Write code</p>",
          },
        },
      ],
    },
  },
});
assert.equal(deshawJobs[0]!.externalId, "5894");
assert.equal(
  deshawJobs[0]!.applyUrl,
  "https://www.deshaw.com/careers/Software-Developer-Intern-New-York-Summer-2027-5894",
);

const shopJobs = parseShopifyCareersHtml(
  `<a href="/careers/software-engineer-intern_a4b8357e-2849-4ef4-80da-459e232f9f16"> intern </a>`,
);
assert.equal(shopJobs[0]!.externalId, "a4b8357e-2849-4ef4-80da-459e232f9f16");
assert.match(shopJobs[0]!.title, /Software Engineer Intern/i);

const gJobs = parseGoogleCareersHtml(
  `ssk='17:12345'<div class="x">jobs/results/12345-software-engineering-intern</div><h3 class="QJPWVe">Software Engineering Intern</h3>`,
);
assert.equal(gJobs[0]!.externalId, "12345");
assert.equal(gJobs[0]!.title, "Software Engineering Intern");

console.log("proprietary adapter tests ok");
