/**
 * Proprietary careers adapter unit tests (no network).
 */
import assert from "node:assert/strict";
import { parseCitadelListingHtml } from "./citadel.js";
import { parseTeslaCareersState } from "./tesla.js";
import { mapBytedancePosts, TIKTOK_PORTAL, BYTEDANCE_PORTAL } from "./bytedance.js";

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

console.log("proprietary adapter tests ok");
