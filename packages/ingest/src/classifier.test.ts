import { isTechInternship, looksRemote } from "./classifier.js";

function assert(cond: boolean, msg: string) {
  if (!cond) throw new Error(msg);
}

assert(isTechInternship("Software Engineer Intern"), "swe intern");
assert(isTechInternship("Machine Learning Co-op"), "ml coop");
assert(isTechInternship("New Grad Software Engineer"), "new grad");
assert(!isTechInternship("Senior Software Engineer"), "senior");
assert(!isTechInternship("Staff Product Manager"), "staff");
assert(looksRemote(["Remote - US"], "Engineer Intern"), "remote loc");
assert(!looksRemote(["New York, NY"], "Engineer Intern"), "nyc");

console.log("classifier ok");
